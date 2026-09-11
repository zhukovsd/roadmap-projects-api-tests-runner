# https://just.systems

default:
    docker compose up -d --build 
    docker logs test-runner -f | jq

post url project="CURRENCY_EXCHANGE":
    curl -X POST "http://localhost:8080/api/tests" \
    -H "Content-Type: application/json" \
    -H "Authorization: $REST_API_KEY" \
    -d '{ "deploy_base_url": "{{url}}", "project_name": "{{project}}" }' | jq

get id:
    curl -X GET "http://localhost:8080/api/tests/{{id}}" -H "Authorization: $REST_API_KEY" | jq
