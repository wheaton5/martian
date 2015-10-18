#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for Redstone main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('RedstoneCtrl', ($scope, $http, $interval) ->

    $scope.FILES = [ 'bam', 'vcf', 'svc', 'svn', 'svp', 'lou' ]
    $scope.FILELABELS = {
        'bam': 'BAM',
        'vcf': 'VCF',
        'svc': 'SvC',
        'svn': 'SvN',
        'svp': 'SvP',
        'lou': 'Loupe',
    }

    $scope.samples = []
    
    $scope.newSample = () ->
        return {
            lenaid:     0,
            version:    'HEAD',
            name:       '',
            files:      {},
        }

    $scope.validate = () ->
        #$scope.output = JSON.stringify($scope.samples, null, '    ')
        #$scope.output = angular.toJson($scope.samples, 4)
        outsamps = []
        for s in $scope.samples
            sfiles = []
            for f in $scope.FILES
                if s.files[f]
                    sfiles.push(f)
            outsamps.push([ s.lenaid, s.version, s.name, sfiles.join('|') ].join(','))
             
        out = {
            from:       $scope.from,
            to:         $scope.to,
            desc:       $scope.desc,
            dtl:        $scope.dtl,
            samples:    outsamps
        }
        $scope.output = angular.toJson(out, 4)

    $scope.addSample = () ->
        newSample = $scope.newSample()
        if $scope.samples.length > 0
            lastSample = $scope.samples[$scope.samples.length-1]
            newSample.lenaid = lastSample.lenaid + 1
            newSample.version = lastSample.version
            newSample.files = _.cloneDeep(lastSample.files)
            newSample.name = lastSample.name
        $scope.samples.push(newSample)
)

# Form validation for integers. 
app.directive('integer', () ->
    return {
        require: 'ngModel',
        link: (scope, elm, attrs, ctrl) ->
            ctrl.$parsers.unshift((viewValue) ->
                if (/^\-?\d+$/.test(viewValue))
                    # it is valid
                    ctrl.$setValidity('integer', true)
                    return parseInt(viewValue, 10)
                else
                    # it is invalid, return undefined (no model update)
                    ctrl.$setValidity('integer', false)
                    return undefined
            )
    }
)
