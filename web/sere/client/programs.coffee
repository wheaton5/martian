#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for SERE programs UI.
#

app = angular.module('app', ['ui.bootstrap'])

callApi = ($scope, $http, $data, $url) ->
    $http.post($url, $data).success((data) ->
        $scope.refreshPrograms()
        if data then window.alert(data.toString())
    )

app.controller('ProgramsCtrl', ($scope, $http, $interval, $modal) ->
    $scope.admin = admin
    $scope.programs = null

    $scope.refreshPrograms = () ->
        $http.get('/api/program/get-programs').success((data) ->
            $scope.programs = data
        )

    $scope.refreshPrograms()

    $scope.isProgramActive = (program) ->
        if program.cycles.length > 0
            cycle = program.cycles[program.cycles.length - 1]
            return cycle.end_date? || cycle.end_date.length == 0
        return false

    $scope.startCycleForm = (program) ->
        modalInstance = $modal.open({
            animation: true,
            templateUrl: 'start_cycle.html',
            controller: 'StartCycleCtrl',
            resolve: {
                program_name: () ->
                    program.name
                cycle_id: () ->
                    program.cycles.length + 1
            }
        })

        modalInstance.result.then((data) ->
            callApi($scope, $http, data, '/api/program/start-cycle')
        , null)

    if admin then $interval((() -> $scope.refreshPrograms()), 5000)
)

app.controller('StartCycleCtrl', ($scope, $modalInstance, program_name, cycle_id) ->
    $scope.data = {program_name: program_name, cycle_id: cycle_id}

    $scope.startCycle = () ->
        $modalInstance.close($scope.data)

    $scope.cancelCycle = () ->
        $modalInstance.dismiss('cancel')
)
