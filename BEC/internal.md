## Build instructions

To build the BEC just run

```bash
docker build . -t thenorstroem/furo-bec:v1.40.0

docker push thenorstroem/furo-bec
```

Build multi arch
```shell
    docker buildx create --use
    
    docker buildx build \
      --platform linux/amd64,linux/arm64 \
      -t thenorstroem/furo-bec:v1.40.1 \
      --push .
```
