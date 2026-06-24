#!/bin/bash
set -e

echo "=========================================="
echo "       Noyo Build Script (Linux)"
echo "=========================================="

# Use version from env or default to git tag/v1.0.0
if [ -z "$VERSION" ]; then
    VERSION=$(git describe --tags --always 2>/dev/null || echo "v1.0.0")
fi

EDITION=$1
if [ -z "$EDITION" ]; then
    if [ -n "$NOYO_PRO_DIR" ]; then
        EDITION="all"
    elif [ -d "../noyo-pro" ]; then
        EDITION="all"
    else
        EDITION="community"
    fi
fi

BUILD_YOLO=${2:-1}
BUILD_ONNXRUNTIME=${3:-1}

if [ "$BUILD_YOLO" != "0" ] && [ "$BUILD_YOLO" != "1" ]; then
    echo "Error: build_yolo must be 0 or 1"
    exit 1
fi

if [ "$BUILD_ONNXRUNTIME" != "0" ] && [ "$BUILD_ONNXRUNTIME" != "1" ]; then
    echo "Error: build_onnxruntime must be 0 or 1"
    exit 1
fi

if [ "$BUILD_YOLO" = "0" ]; then
    BUILD_ONNXRUNTIME=0
fi

if [ "$BUILD_YOLO" = "1" ] && [ "$BUILD_ONNXRUNTIME" = "0" ]; then
    echo "Error: build_yolo=1 requires build_onnxruntime=1"
    exit 1
fi

echo "Target Edition: $EDITION"
echo "Building version: $VERSION"
echo "Build YOLO: $BUILD_YOLO"
echo "Build ONNX Runtime: $BUILD_ONNXRUNTIME"

build_edition() {
    local edition_name=$1
    local bin_suffix=$2
    echo "=========================================="
    echo "       Building $edition_name Edition"
    echo "=========================================="
    
    export NOYO_EDITION=$edition_name
    export NOYO_BUILD_YOLO=$BUILD_YOLO

    echo "[1/3] Building Frontend..."
    cd frontend
    npm install
    npm run build
    cd ..

    echo "[2/3] Skipping copy (Vite builds to backend/dist directly)..."

    echo "[3/3] Building Backend..."
    cd backend
    export CGO_ENABLED=0
    if [ "$edition_name" = "pro" ] && [ "$BUILD_YOLO" = "1" ] && [ "$BUILD_ONNXRUNTIME" = "1" ]; then
        export CGO_ENABLED=1
    fi
    export GOOS=linux
    export GOARCH=amd64

    go build -ldflags "-w -s -X 'noyo/core/system.Version=$VERSION'" -o "noyo-linux-amd64$bin_suffix" .
    cd ..
    
    echo "Finished building $edition_name edition -> backend/noyo-linux-amd64$bin_suffix"
}

if [ "$EDITION" = "all" ]; then
    build_edition "community" ""
    build_edition "pro" "-pro"
elif [ "$EDITION" = "pro" ]; then
    build_edition "pro" "-pro"
else
    build_edition "community" ""
fi

echo "=========================================="
echo "Build Success!"
if [ "$EDITION" = "all" ]; then
    echo "Binaries: backend/noyo-linux-amd64 and backend/noyo-linux-amd64-pro"
elif [ "$EDITION" = "pro" ]; then
    echo "Binary: backend/noyo-linux-amd64-pro"
else
    echo "Binary: backend/noyo-linux-amd64"
fi
echo "=========================================="
