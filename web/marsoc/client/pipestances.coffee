#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for Marsoc pipestances UI.
#

app = angular.module('app', ['ui.bootstrap'])

callApiWithConfirmation = ($scope, $http, $p, $url) ->
    $scope.showbutton = false
    psid = window.prompt("Please type the sample ID to confirm")
    if psid == $p.psid
        $http.post($url, { fcid: $p.fcid, pipeline: $p.pipeline, psid: $p.psid }).success((data) ->
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
            $scope.pipestances = _.sortBy(data, (p) ->
                [p.fcid, p.pipeline, p.psid, p.state]
            )
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

    $scope.filterPipestance = (p) ->
        for prop in ['fcid', 'pipeline', 'psid', 'state']
            if $scope[prop] && $scope[prop] != p[prop]
                return false
        return true

    $scope.archivePipestance = (p) ->
        callApiWithConfirmation($scope, $http, p, '/api/archive-sample')

    $scope.wipePipestance = (p) ->
        callApiWithConfirmation($scope, $http, p, '/api/wipe-sample')

    $scope.killPipestance = (p) ->
        callApiWithConfirmation($scope, $http, p, '/api/kill-sample')

    $scope.unfailPipestance = (p) ->
        callApiWithConfirmation($scope, $http, p, '/api/restart-sample')

    $scope.capitalize = (str) ->
       return str[0].toUpperCase() + str[1..]

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