#!/usr/bin/env bash
set -euo pipefail
cd /usr/share/elasticsearch
umask 027
if [ ! -f config/certs/ca/ca.crt ]; then
  bin/elasticsearch-certutil ca --silent --pem -out config/certs/ca.zip
  unzip -q config/certs/ca.zip -d config/certs
fi
if [ ! -f config/certs/elasticsearch/elasticsearch.crt ]; then
  bin/elasticsearch-certutil cert --silent --pem --ca-cert config/certs/ca/ca.crt --ca-key config/certs/ca/ca.key --name elasticsearch --dns elasticsearch,localhost --ip 127.0.0.1 -out config/certs/es.zip
  unzip -q config/certs/es.zip -d config/certs
fi
chown -R root:0 config/certs
find config/certs -type d -exec chmod 755 {} \;
find config/certs -type f -name '*.crt' -exec chmod 644 {} \;
find config/certs -type f -name '*.key' -exec chmod 640 {} \;

# Certificate generation precedes ES startup; provisioning waits for its HTTP API.
elastic_password=$(cat /run/secrets/elastic_password)
until curl --silent --fail --cacert config/certs/ca/ca.crt -u "elastic:$elastic_password" https://elasticsearch:9200 >/dev/null; do sleep 3; done
request() { curl --silent --show-error --fail --cacert config/certs/ca/ca.crt -u "elastic:$elastic_password" -H 'Content-Type: application/json' "$@"; }
for index in access-history-v1 access-monitor-logs-v1; do
  status=$(curl --silent --cacert config/certs/ca/ca.crt -u "elastic:$elastic_password" -o /dev/null -w '%{http_code}' "https://elasticsearch:9200/$index")
  if [ "$status" = 404 ]; then
    request -X PUT "https://elasticsearch:9200/$index" --data-binary "@/setup/$index.json"
  elif [ "$status" != 200 ]; then
    echo "Cannot inspect existing index $index (HTTP $status)" >&2; exit 1
  fi
done
request -X PUT https://elasticsearch:9200/_security/role/access_shipper -d '{"cluster":["monitor"],"indices":[{"names":["access-history-v1","access-monitor-logs-v1"],"privileges":["index"]}]}'
request -X PUT https://elasticsearch:9200/_security/role/access_reader -d '{"cluster":["monitor"],"indices":[{"names":["access-history-v1","access-monitor-logs-v1"],"privileges":["read","view_index_metadata"]}]}'
# Password files are generated as hexadecimal text, so they are safe JSON strings.
for pair in 'filebeat:access_shipper:filebeat_password' 'grafana_reader:access_reader:grafana_reader_password'; do
 IFS=: read -r user role secret <<< "$pair"
 password=$(cat "/run/secrets/$secret")
 [[ "$password" =~ ^[a-f0-9]{32,128}$ ]] || { echo 'Use generated hexadecimal service passwords' >&2; exit 1; }
 request -X PUT "https://elasticsearch:9200/_security/user/$user" -d "{\"password\":\"$password\",\"roles\":[\"$role\"]}"
done
echo 'Elasticsearch provisioning complete'
