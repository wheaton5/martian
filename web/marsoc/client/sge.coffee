#
# Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
#
# Angular controllers for marsoc qweb main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('QWebCtrl', ($scope, $http, $interval) ->
    $scope.qstat = null
    $http.get('/api/qstat').success((data) ->
        console.log(data)
        if typeof(data) == "string"
            window.alert(data)
        else
            $scope.qstat = data
    )
)
