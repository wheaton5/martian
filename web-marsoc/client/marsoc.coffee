#
# Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
#
# Angular controllers for mario runner main UI.
#

app = angular.module('app', ['ui.bootstrap'])
app.filter('momentFormat',  () -> (time, fmt) -> moment(time).format(fmt)
).filter('momentTimeAgo', () -> (time) -> moment(time).fromNow()
).filter('flowcellFront', () -> (fcid) -> fcid.substr(0,5)
).filter('flowcellBack',  () -> (fcid) -> fcid.substr(5,4)
).filter('runDuration',   () -> (run) ->
    if run.completeTime
        diff = moment(run.completeTime).diff(run.startTime, 'hours')
    else
        diff = moment(run.touchTime).diff(run.startTime, 'hours')
    diff || '<1'
)

app.controller('MarioRunCtrl', ($scope, $http, $interval) ->
    $scope.admin = admin
    $scope.urlprefix = if admin then '/admin' else ''

    $scope.selrun = null
    $scope.sampi = 0
    $scope.samples = null
    $scope.showbutton = true
    
    $http.get('/api/get-runs').success((data) ->
        $scope.runs = data
        $scope.runTable = _.indexBy($scope.runs, 'fcid')
    )

    $scope.refreshRuns = () ->
        $http.get('/api/get-runs').success((runs) ->
            for run in runs
                $scope.runTable[run.fcid].preprocess = run.preprocess
                $scope.runTable[run.fcid].state = run.state
            $http.post('/api/get-samples', { fcid: $scope.selrun?.fcid }).success((data) ->
                $scope.samples = data
                $scope.showbutton = true
            )
        )

    $scope.selectRun = (run) ->
        $scope.samples = null
        for r in $scope.runs
            r.selected = false
        $scope.selrun = run
        $scope.selrun.selected = true
        $http.post('/api/get-samples', { fcid: $scope.selrun?.fcid }).success((data) ->
            $scope.samples = data
        )
        $http.post('/api/get-callsrc', { fcid: $scope.selrun?.fcid }).success((data) ->
            $scope.selrun?.callsrc = data
        )

    $scope.invokePreprocess = () ->
        $scope.showbutton = false
        $http.post('/api/invoke-preprocess', { fcid: $scope.selrun.fcid }).success((data) ->
            $scope.refreshRuns()
            if data then window.alert(data.toString())
        )

    $scope.invokeAnalysis = () ->
        $scope.showbutton = false
        $http.post('/api/invoke-analysis', { fcid: $scope.selrun.fcid }).success((data) ->
            $scope.refreshRuns()
            if data then window.alert(data.toString())
        )

    $scope.archiveSamples = () ->
        $scope.showbutton = false
        $http.post('/api/archive-fcid-samples', { fcid: $scope.selrun.fcid }).success((data) ->
            $scope.refreshRuns()
            if data then window.alert(data.toString())
        )

    $scope.allDone = () ->
        _.every($scope.samples, (s) -> s.psstate == 'complete')
        
    # Only admin pages get auto-refresh.
    if admin then $interval((() -> $scope.refreshRuns()), 5000)
)

