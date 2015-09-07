#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for martian runner main UI.
#

app = angular.module('app', ['ui.bootstrap'])
app.filter('formatNumber', () -> (n, d) -> Humanize.formatNumber(n, d))
app.filter('intComma', () -> (n) -> Humanize.intComma(n))

app.controller('MartianRunCtrl', ($scope, $http, $interval) ->
    $scope.pstances = null
    
    $http.get('/api/get-submissions').success((data) ->
        for d in data
            d.display_date = d.date.substring(0,10)
            d.display_domain = d.source.split("@")[0]
            d.display_user = d.source.split("@")[1]
        $scope.subs = data
    )
)

