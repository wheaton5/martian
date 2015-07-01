#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for SERE program UI.
#

app = angular.module('app', ['ui.bootstrap'])

callApi = ($scope, $http, $data, $url) ->
    $scope.showbutton = false
    $http.post($url, $data).success((data) ->
        $scope.refreshProgram()
        if data then window.alert(data.toString())
    )

getTests = (tests, state) ->
    test for test in tests when test.state == state

app.controller('ProgramCtrl', ($scope, $http, $interval, $modal) ->
    $scope.admin = admin
    $scope.program_name = program_name
    $scope.cycle_id = cycle_id
    $scope.showbutton = true

    $scope.program = null
    $scope.cycle = null
    $scope.packages = null

    $scope.refreshProgram = () ->
        $http.get('/api/program/' + $scope.program_name + '/' + $scope.cycle_id).success((data) ->
            $scope.program = data
            $scope.cycle = data.cycles[0]
            $scope.showbutton = true
        )
        $http.get('/api/manage/get-items').success((data) ->
            $scope.packages = (p for p in data.packages when p.state == 'complete')
        )

    $scope.refreshProgram()

    $scope.isCycleActive = () ->
        if $scope.cycle
            return $scope.cycle.end_date.length == 0
        return false

    $scope.endCycle = () ->
        $scope.showbutton = false
        value = window.confirm('Are you sure you want to end this cycle?')
        if value
            data = {program_name: $scope.program.name, cycle_id: $scope.cycle.id}
            callApi($scope, $http, data, '/api/program/end-cycle')
        else
            window.alert('This cycle is still active!')
            $scope.showbutton = true

    $scope.someTests = (round, state) ->
        tests = getTests(round.tests, state)
        tests.length > 0

    $scope.invoke = (round) ->
        $scope.pipestancesForm(round, 'Invoke Pipestances', 'ready')

    $scope.unfail = (round) ->
        $scope.pipestancesForm(round, 'Unfail pipestances', 'failed')

    $scope.kill = (round) ->
        $scope.pipestancesForm(round, 'Kill Pipestances', 'running')

    $scope.pipestancesForm = (round, title, state) ->
        modalInstance = $modal.open({
            animation: true,
            templateUrl: 'pipestances.html',
            controller: 'PipestancesCtrl',
            resolve: {
                tests: () ->
                    getTests(round.tests, state)
                title: () ->
                    title
                state: () ->
                    state
            }
        })

        modalInstance.result.then((data) ->
            tests = (test for test in data.tests when test.selected)
            switch data.state
                when 'ready'
                    url = '/api/test/invoke-pipestances'
                when 'failed'
                    url = '/api/test/restart-pipestances'
                when 'running'
                    url = '/api/test/kill-pipestances'
            callApi($scope, $http, tests, url)
        , null)

    $scope.startRoundForm = () ->
        modalInstance = $modal.open({
            animation: true,
            templateUrl: 'start_round.html',
            controller: 'StartRoundCtrl',
            resolve: {
                program_name: () ->
                    $scope.program.name
                cycle_id: () ->
                    $scope.cycle.id
                round_id: () ->
                    $scope.cycle.rounds.length + 1
                packages: () ->
                    $scope.packages
            }
        })

        modalInstance.result.then((data) ->
            callApi($scope, $http, data, '/api/cycle/start-round')
        , null)

    if admin then $interval((() -> $scope.refreshProgram()), 5000)
)

app.controller('StartRoundCtrl', ($scope, $modalInstance, program_name, cycle_id, round_id, packages) ->
    $scope.data = {program_name: program_name, cycle_id: cycle_id, round_id: round_id}
    $scope.packages = packages

    $scope.formatPackage = (p) ->
        p.name + ' : '+ p.target + ' : ' + p.mro_version

    $scope.startRound = () ->
        $modalInstance.close($scope.data)

    $scope.cancelRound = () ->
        $modalInstance.dismiss('cancel')
)

app.controller('PipestancesCtrl', ($scope, $modalInstance, tests, title, state) ->
    $scope.tests = tests
    $scope.title = title
    $scope.state = state

    $scope.selectAll = () ->
        for test in $scope.tests
            test.selected = !test.selected

    $scope.startPipestances = () ->
        data = {tests: $scope.tests, state: $scope.state}
        $modalInstance.close(data)

    $scope.cancelPipestances = () ->
        $modalInstance.dismiss('cancel')
)
