set shell := ["/usr/bin/env", "bash", "-c"]
set dotenv-load

default: install

fmt:
    go fmt ./...

run:
    go run cmd/main.go

build:
    go build -o ops cmd/main.go

lint:
    golangci-lint run

tidy:
    go mod tidy

add PACKAGE:
    go get -u {{PACKAGE}}

install:
    go build -o $HOME/go/bin/ops -ldflags "-X github.com/0p5dev/ops/internal/config.SupabaseURL=$SUPABASE_URL -X github.com/0p5dev/ops/internal/config.SupabaseKey=$SUPABASE_API_KEY -X main.Version=v0.1.19" ./cmd

release *MESSAGE:
    #!/usr/bin/env bash
    set -euo pipefail
    LATEST_TAG=$(curl -L \
      -H "Accept: application/vnd.github+json" \
      -H "Authorization: Bearer $GITHUB_ACCESS_TOKEN" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      https://api.github.com/repos/0p5dev/ops/releases/latest | jq -r '.tag_name')
    
    echo "Latest release tag: $LATEST_TAG"
    
    VERSION=${LATEST_TAG#v}
    MAJOR=$(echo $VERSION | cut -d. -f1)
    MINOR=$(echo $VERSION | cut -d. -f2)
    PATCH=$(echo $VERSION | cut -d. -f3)
    
    NEW_PATCH=$((PATCH + 1))
    NEW_TAG="v${MAJOR}.${MINOR}.${NEW_PATCH}"

    echo "Pushing to main branch"
    git add -A
    git commit -m "{{MESSAGE}}"
    git push origin main
    
    echo "Creating new tag: $NEW_TAG with message: {{MESSAGE}}"
    git tag -a "$NEW_TAG" -m "{{MESSAGE}}"
    git push origin "$NEW_TAG"