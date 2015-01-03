#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for martian mrv main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('MartianRunCtrl', ($scope, $http, $interval) ->
    $scope.pipestances = []
    $scope.config = {}

    $http.get('/api/get-pipestances').success((data) ->
        $scope.pipestances = data.pipestances
        $scope.config = data.config
    )
)
