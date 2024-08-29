#!/bin/bash

usage() {
    echo "Usage: ./dockerbuild.sh target version"
    echo "target: game, gate, all"
}

if [ $# != 2 ]; then
    usage
    exit
fi

case $1 in
"game")
    docker build -t panshiqu/game_server:latest -t panshiqu/game_server:1 -t panshiqu/game_server:1.$2 \
    --build-arg SERVER=game_server --build-arg VERSION=v1.$2 .
;;
"gate")
    docker build -t panshiqu/gate_server:latest -t panshiqu/gate_server:1 -t panshiqu/gate_server:1.$2 \
    --build-arg SERVER=gate_server --build-arg VERSION=v1.$2 .
;;
"all")
    docker build -t panshiqu/game_server:latest -t panshiqu/game_server:1 -t panshiqu/game_server:1.$2 \
    --build-arg SERVER=game_server --build-arg VERSION=v1.$2 .

    docker build -t panshiqu/gate_server:latest -t panshiqu/gate_server:1 -t panshiqu/gate_server:1.$2 \
    --build-arg SERVER=gate_server --build-arg VERSION=v1.$2 .
;;
*)
    usage
;;
esac
