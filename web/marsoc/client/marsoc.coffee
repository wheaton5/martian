#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for mario runner main UI.
#

actualSeconds = (run) ->
    if run.completeTime
        d = moment(run.completeTime).diff(run.startTime)
    else
        d = moment(run.touchTime).diff(run.startTime)
    return moment.duration(d / 1000, 'seconds')

predictedSeconds = (run) ->
    reads = run.runinfoxml.Run.Reads.Reads
    total = _.reduce(reads, (sum, read) -> 
        sum + read.NumCycles
    , 0)
    if run.seqcerName.indexOf("hiseq") == 0
        d = 314 * total + 30960
    else
        d = 249 * total + 6060
    return moment.duration(d, 'seconds')
    
app = angular.module('app', ['ui.bootstrap'])
app.filter('momentFormat',  () -> (time, fmt) -> moment(time).format(fmt)
).filter('momentTimeAgo', () -> (time) -> moment(time).fromNow()
).filter('flowcellFront', () -> (fcid) -> fcid.substr(0,5)
).filter('flowcellBack',  () -> (fcid) -> fcid.substr(5,4)
).filter('cycleInfo',    () -> (selrun) ->
    reads = selrun.runinfoxml.Run.Reads.Reads
    readLens = _.map(reads, (read) -> read.NumCycles).join(", ")
    total = _.reduce(reads, (sum, read) -> 
        sum + read.NumCycles
    , 0)
    "#{readLens} (#{total})"
).filter('runDuration', () -> (run) ->
    dact = actualSeconds(run)
    if not dact? then return '<1' 
    dpred = predictedSeconds(run)
    pctg = Math.floor(dact / dpred * 100.0)
    "#{dact.hours() + 24 * dact.days()}h #{dact.minutes()}m (#{pctg}%)" 
).filter('runPrediction', () -> (run) ->
    dact = actualSeconds(run)
    dpred = predictedSeconds(run)
    eta = moment(run.startTime).add(dpred).format("ddd MMM D, h:mm a")
    "#{dpred.hours() + 24 * dpred.days()}h #{dpred.minutes()}m (#{eta})"
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

    $scope.unfailSamples = () ->
	$scope.showbutton = false
        $http.post('/api/restart-fcid-samples', { fcid: $scope.selrun.fcid }).success((data) ->
            $scope.refreshRuns()
            if data then window.alert(data.toString())
        )

    $scope.allDone = () ->
        _.every($scope.samples, (s) -> s.psstate == 'complete')

    $scope.allFail = () ->
        _.every($scope.samples, (s) -> s.psstate == 'failed')
        
    # Only admin pages get auto-refresh.
    if admin then $interval((() -> $scope.refreshRuns()), 5000)
)

