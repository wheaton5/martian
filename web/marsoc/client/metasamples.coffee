#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for mario runner main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('MarioRunCtrl', ($scope, $http, $interval) ->
    $scope.admin = admin
    $scope.urlprefix = if admin then '/admin' else ''

    $scope.selsample = null
    $scope.showbutton = true
   
    $http.get('/api/get-metasamples').success((data) ->
        $scope.samples = data
    )

    $scope.selectSample = (sample) ->
        $scope.selsample = sample
        for s in $scope.samples
            s.selected = false
        $scope.selsample.selected = true
        $http.post('/api/get-metasample-callsrc', { id: $scope.selsample?.id.toString() }).success((data) ->
            if $scope.selsample? then  _.assign($scope.selsample, data)
        )

    $scope.invokeAnalysis = () ->
        $scope.showbutton = false
        $http.post('/api/invoke-metasample-analysis', { id: "" + $scope.selsample?.id.toString() }).success((data) ->
            if data then window.alert(data.toString())
        )
)
