#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for mario mrv main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('MarioRunCtrl', ($scope, $http, $interval) ->
    $scope.pipestances = []
    $scope.usermap = {}

    $http.get('/api/get-pipestances').success((data) ->
        $scope.pipestances = data.pipestances
        $scope.usermap = data.usermap
    )
)
