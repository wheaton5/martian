#!/bin/bash

if [ -n "$(gofmt -l src/martian)" ]; then
    echo "Go code is not formatted:"
    gofmt -d src/martian
    exit 1
fi

if [ -n "$(gofmt -l src/github.com/10XDev/martian-public/src/martian)" ]; then
    echo "Go code is not formatted:"
    gofmt -d src/github.com/10XDev/martian-public/src/martian
    exit 1
fi
