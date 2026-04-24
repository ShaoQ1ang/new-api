# web-v2 Docker notes

## Quick preview

Run the backend on the host machine, then start the frontend container:

```bash
cd /Users/niuyouguo/go/src/new-api
SESSION_SECRET=dev-web-v2-session-secret MEMORY_CACHE_ENABLED=true go run .
```

```bash
cd /Users/niuyouguo/go/src/new-api/web-v2
HTTP_PROXY=http://127.0.0.1:7897 \
HTTPS_PROXY=http://127.0.0.1:7897 \
ALL_PROXY=socks5://127.0.0.1:7897 \
docker compose up --build -d
```

The frontend is available at `http://127.0.0.1:3001`.

## Full stack compose

When Docker registry access is healthy, you can run frontend and backend together:

```bash
cd /Users/niuyouguo/go/src/new-api/web-v2
HTTP_PROXY=http://127.0.0.1:7897 \
HTTPS_PROXY=http://127.0.0.1:7897 \
ALL_PROXY=socks5://127.0.0.1:7897 \
docker compose -f docker-compose.fullstack.yml up --build -d
```

Ports:

- Backend: `http://127.0.0.1:3000`
- Frontend: `http://127.0.0.1:3001`
