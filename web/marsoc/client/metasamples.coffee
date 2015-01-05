#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for martian runner main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('MartianRunCtrl', ($scope, $http, $interval) ->
    $scope.admin = admin
    $scope.urlprefix = if admin then '/admin' else ''

    $scope.selsample = null
    $scope.showbutton = true
   
    $http.get('/api/get-metasamples').success((data) ->
        $scope.samples = data
    )

    $scope.refreshSamples = () ->
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
        $http.post('/api/invoke-metasample-analysis', { id: $scope.selsample?.id.toString() }).success((data) ->
            $scope.refreshSamples()
            if data then window.alert(data.toString())
        )

    $scope.archiveSample = () ->
        $scope.showbutton = false
        $http.post('/api/archive-metasample', { id: $scope.selsample?.id.toString() }).success((data) ->
            $scope.refreshSamples()
            if data then window.alert(data.toString())
        )

    $scope.unfailSample = () ->
        $scope.showbutton = false
        $http.post('/api/restart-metasample-analysis', { id: $scope.selsample?.id.toString() }).success((data) ->
            $scope.refreshSamples()
            if data then window.alert(data.toString())
        )

    # Only admin pages get auto-refresh.
    if admin then $interval((() -> $scope.refreshSamples()), 5000)
)
