#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for Marsoc pipestances UI.
#

app = angular.module('app', ['ui.bootstrap'])

callApiWithConfirmation = ($scope, $http, $url) ->
    $scope.showbutton = false
    psid = window.prompt("Please type the sample ID to confirm")
    if psid == $scope.selps.psid
        $http.post($url, { psid: $scope.selps.psid }).success((data) ->
            $scope.refreshPipestances()
            if data then window.alert(data.toString())
        )
    else
        window.alert("Incorrect sample ID")

app.controller('PipestancesCtrl', ($scope, $http, $interval) ->
    $scope.admin = admin
    $scope.urlprefix = if admin then '/admin' else ''
    $scope.autoinvoke = { button: true, state: false }

    $scope.showbutton = true
    $scope.fcid = null
    $scope.pipeline = null
    $scope.psid = null
    $scope.state = "running"

    $scope.refreshPipestances = () ->
        $http.get('/api/get-pipestances').success((data) ->
            $scope.pipestances = data
            fcids = []
            pipelines = []
            psids = []
            for p in $scope.pipestances
                fcids.push p.fcid
                pipelines.push p.pipeline
                psids.push p.psid
            $scope.fcids = _.uniq(fcids)
            $scope.pipelines = _.uniq(pipelines)
            $scope.psids = _.uniq(psids)
            $scope.showbutton = true
        )
        $http.get('/api/get-auto-invoke-status').success((data) ->
            $scope.autoinvoke.state = data.state
            $scope.autoinvoke.button = true
        )

    $scope.refreshPipestances()

    $scope.archivePipestance = () ->
        callApiWithConfirmation($scope, $http, '/api/archive-sample')

    $scope.wipePipestance = () ->
        callApiWithConfirmation($scope, $http, '/api/wipe-sample')

    $scope.killPipestance = () ->
        callApiWithConfirmation($scope, $http, '/api/kill-sample')

    $scope.unfailPipestance = () ->
        callApiWithConfirmation($scope, $http, '/api/unfail-sample')

    $scope.getAutoInvokeClass = () ->
    if $scope.autoinvoke.state
            return "complete"
        else
            return "failed"

    $scope.setAutoInvoke = () ->
        $scope.autoinvoke.button = false
        $http.post('/api/set-auto-invoke-status', { state: !$scope.autoinvoke.state }).success((data) ->
            $scope.refreshPipestances()
            if data then window.alert(data.toString())
        )

    # Only admin pages get auto-refresh.
    if admin then $interval((() -> $scope.refreshPipestances()), 5000)
)