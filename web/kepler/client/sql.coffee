#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controller for Kepler UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('SqlCtrl', ($scope, $http, $interval) ->
    $scope.res = null
    $scope.query = null
    $scope.error = null

    $scope.getResult = () ->
        $http.post('/api/get-sql', { query: $scope.query }).success((data) ->
            if data['error']
                $scope.error = data['error']
                $scope.res = null
            else
                $scope.res = data
                $scope.error = null
        ).error(() ->
            console.log('Server responded with an error for /api/get-sql.')
        )

    $scope.clearResult = () ->
        $scope.res = null
        $scope.error = null
)