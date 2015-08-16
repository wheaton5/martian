#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for martian runner main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('MartianRunCtrl', ($scope, $http, $interval) ->
    $scope.pstances = null
    
    $http.get('/api/get-pipestances').success((data) ->
        for d in data
            d.display_date = d.date.substring(0,10)
        $scope.pstances = data
    )
)

