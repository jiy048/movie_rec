FROM docker.elastic.co/elasticsearch/elasticsearch:7.9.3

ENV discovery.type=single-node \
    xpack.security.enabled=false \
    ES_JAVA_OPTS="-Xms512m -Xmx512m"

EXPOSE 9200