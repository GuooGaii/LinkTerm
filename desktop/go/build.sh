#!/bin/bash

echo "Building Terminal Client..."

# Build for current platform
go build -o terminal

if [ $? -eq 0 ]; then
    echo "Build successful! Run ./terminal to start."
    chmod +x terminal
else
    echo "Build failed!"
    exit 1
fi
