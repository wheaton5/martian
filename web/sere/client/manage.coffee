#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for SERE manage UI.
#

app = angular.module('app', ['ui.bootstrap'])

callApi = ($scope, $http, $data, $url) ->
    $http.post($url, $data).success((data) ->
        $scope.refreshItems()
        if data then window.alert(data.toString())
    )

capitalize = (str) ->
    return str[0].toUpperCase() + str[1..]

app.controller('ManageCtrl', ($scope, $http, $interval, $modal) ->
    $scope.admin = admin
    $scope.data = null

    $scope.categories = ['lena', 'standard', 'fuzzer', 'custom']
    $scope.cols = {
        programs: ['name', 'battery'],
        batteries: ['name', 'tests'],
        tests: ['name', 'category', 'id'],
        packages: ['name', 'target', 'build_date', 'mro_version', 'state'],
    }
    $scope.types = _.keys($scope.cols)
    $scope.type = 'programs'

    $scope.refreshItems = () ->
        $http.get('/api/manage/get-items').success((data) ->
            $scope.data = data
        )

    $scope.refreshItems()

    $scope.getName = (prop) ->
        if typeof prop is 'object'
            # Get name key if property is object
            if 'name' of prop
                return prop.name

            # Array formats as comma-separated list of names
            values = []
            for value in prop
                values.push $scope.getName(value)
            return values.join(', ')
        return prop

    $scope.manageItemForm = (action) ->
        modalInstance = $modal.open({
            animation: true,
            templateUrl: 'manage_item.html',
            controller: 'ManageItemCtrl',
            resolve : {
                title: () ->
                    capitalize(action) + ' ' + capitalize($scope.type)
                action: () ->
                    action
                type: () ->
                    $scope.type
                categories: () ->
                    $scope.categories
                data: () ->
                    $scope.data
            }
        })

        modalInstance.result.then((item) ->
            switch $scope.type
                when 'programs'
                    data = {name: item.name, battery: item.battery.name}
                    url = '/api/manage/' + action + '-program'
                when 'batteries'
                    data = {name: item.name, tests: test.name for test in item.tests}
                    url = '/api/manage/' + action + '-battery'
                when 'tests'
                    data = {name: item.name, category: item.category, id: item.id}
                    url = '/api/manage/' + action + '-test'
                when 'packages'
                    data = {name: item.name, target: item.target}
                    url = '/api/manage/' + action + '-package'
            callApi($scope, $http, data, url)
        , null)

    if admin then $interval((() -> $scope.refreshItems()), 5000)
)

app.controller('ManageItemCtrl', ($scope, $modalInstance, title, action, type, categories, data) ->
    $scope.title = title
    $scope.action = action
    $scope.type = type
    $scope.categories = categories
    $scope.data = data
    $scope.names = (item.name for item in data[type])
    $scope.item = {}

    $scope.manageItem = () ->
        $modalInstance.close($scope.item)

    $scope.cancelItem = () ->
        $modalInstance.dismiss('cancel')
)
