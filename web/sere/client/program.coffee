#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for SERE program UI.
#

app = angular.module('app', ['ui.bootstrap'])

callApi = ($scope, $http, $data, $url) ->
    $http.post($url, $data).success((data) ->
        $scope.refreshProgram()
        if data then window.alert(data.toString())
    )

app.controller('ProgramCtrl', ($scope, $http, $interval, $modal) ->
    $scope.admin = admin
    $scope.program_name = program_name
    $scope.cycle_id = cycle_id

    $scope.refreshProgram = () ->
        $http.get('/api/program/' + $scope.program_name + '/' + $scope.cycle_id.toString()).success((data) ->
            $scope.program = data
            $scope.cycle = data.cycles[0]
        )
        $http.get('/api/manage/get-items').success((data) ->
            $scope.packages = (p for p in data.packages when p.state == 'complete')
        )

    $scope.refreshProgram()

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
