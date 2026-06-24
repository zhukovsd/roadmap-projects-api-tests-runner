# https://just.systems

default:
    docker compose up -d --build 
    docker logs test-runner -f | jq
