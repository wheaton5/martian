#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for martian runner main UI.
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
        d = 379 * (total-12) + 21513
    else
        d = 268 * total + 7080
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

callApiWithConfirmation = ($scope, $http, $url) ->
    $scope.showbutton = false
    fcid = window.prompt("Please type the flowcell ID to confirm")
    if fcid == $scope.selrun.fcid
        callApi($scope, $http, $url)
    else
        window.alert("Incorrect flowcell ID")

callApi = ($scope, $http, $url) ->
    $scope.showbutton = false
    $http.post($url, { fcid: $scope.selrun.fcid }).success((data) ->
        $scope.refreshRuns()
        if data then window.alert(data.toString())
    )

app.controller('MartianRunCtrl', ($scope, $http, $interval) ->
    $scope.admin = admin
    $scope.urlprefix = if admin then '/admin' else ''

    $scope.selrun = null
    $scope.sampi = 0
    $scope.samples = null
    $scope.showbutton = true
    $scope.autoinvoke = { button: true, state: false }
    
    $http.get('/api/get-runs').success((data) ->
        $scope.runs = data
        $scope.runTable = _.indexBy($scope.runs, 'fcid')
    )

    $http.get('/api/get-auto-invoke-status').success((data) ->
        $scope.autoinvoke.state = data.state
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
        $http.get('/api/get-auto-invoke-status').success((data) ->
            $scope.autoinvoke.state = data.state
            $scope.autoinvoke.button = true
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
        callApi($scope, $http, '/api/invoke-preprocess')

    $scope.wipePreprocess = () ->
        callApiWithConfirmation($scope, $http, '/api/wipe-preprocess')

    $scope.killPreprocess = () ->
        callApiWithConfirmation($scope, $http, '/api/kill-preprocess')

    $scope.archivePreprocess = () ->
        callApiWithConfirmation($scope, $http, '/api/archive-preprocess')

    $scope.invokeAnalysis = () ->
        callApi($scope, $http, '/api/invoke-analysis')

    $scope.archiveSamples = () ->
        callApiWithConfirmation($scope, $http, '/api/archive-fcid-samples')

    $scope.wipeSamples = () ->
        callApiWithConfirmation($scope, $http, '/api/wipe-fcid-samples')

    $scope.killSamples = () ->
        callApiWithConfirmation($scope, $http, '/api/kill-fcid-samples')

    $scope.unfailSamples = () ->
        callApi($scope, $http, '/api/restart-fcid-samples')

    $scope.allDone = () ->
        _.every($scope.samples, (s) -> s.psstate == 'complete')

    $scope.someFail = () ->
        _.some($scope.samples, (s) -> s.psstate == 'failed')

    $scope.someRunning = () ->
        _.some($scope.samples, (s) -> s.psstate == 'running')

    $scope.getAutoInvokeClass = () ->
        if $scope.autoinvoke.state
            return "complete"
        else
            return "failed"

    $scope.setAutoInvoke = () ->
        $scope.autoinvoke.button = false
        $http.post('/api/set-auto-invoke-status', { state: !$scope.autoinvoke.state }).success((data) ->
            $scope.refreshRuns()
            if data then window.alert(data.toString())
        )
        
    # Only admin pages get auto-refresh.
    if admin then $interval((() -> $scope.refreshRuns()), 5000)
)

