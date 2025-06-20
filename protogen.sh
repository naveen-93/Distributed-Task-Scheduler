#!/bin/zsh

set -e  # Exit immediately if a command fails

# === Step 1: Install protoc-gen-go and protoc-gen-go-grpc ===
echo "🔧 Installing Go protobuf plugins..."

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# === Step 2: Add GOPATH/bin to PATH if not already there ===
GOBIN="$(go env GOPATH)/bin"

if [[ ":$PATH:" != *":$GOBIN:"* ]]; then
  echo "🔧 Adding $GOBIN to PATH in ~/.zshrc..."
  echo "export PATH=\"\$PATH:$GOBIN\"" >> ~/.zshrc
  export PATH="$PATH:$GOBIN"
else
  echo "✅ $GOBIN already in PATH"
fi

# === Step 3: Create gen directory if it doesn't exist ===
mkdir -p gen

# === Step 4: Run protoc ===
echo "🚀 Generating Go code from proto/scheduler.proto..."
protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  proto/scheduler.proto

echo "✅ Protobuf code generated in ./gen/"
