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
        #$scope.runTable = _.indexBy($scope.runs, 'fcid')
    )

    $scope.selectSample = (sample) ->
        $scope.selsample = sample
        $scope.selsample.selected = true
        console.log($scope.selsample.id)
        $http.post('/api/get-metasample-callsrc', { id: "" + $scope.selsample?.id }).success((data) ->
            console.log(data)
            $scope.selsample?.callsrc = data
        )

    $scope.invokeAnalysis = () ->
        $scope.showbutton = false
        $http.post('/api/invoke-analysis', { fcid: $scope.selrun.fcid }).success((data) ->
            $scope.refreshRuns()
            if data then window.alert(data.toString())
        )
)

