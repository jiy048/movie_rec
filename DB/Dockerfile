FROM docker.elastic.co/elasticsearch/elasticsearch:7.9.3

# dup with docker-compose. docker-compose.yml will override this. might be useful for prod 
ENV discovery.type=single-node \
    xpack.security.enabled=false \
    ES_JAVA_OPTS="-Xms512m -Xmx512m"
    