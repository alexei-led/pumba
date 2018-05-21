#!/usr/bin/env bats

@test "Pumba Help" {
    run pumba --help
    [ $status -eq 0 ]
}

@test "Pumba Help Command" {
    run pumba help
    [ $status -eq 0 ]
}

@test "Pumba Version" {
    ver=$(cat VERSION)
    run pumba --version
    [ $status -eq 0 ]
    [[ $output == *"$ver"* ]]
}
