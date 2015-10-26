#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for Redstone main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('RedstoneCtrl', ($scope, $http, $interval) ->
    $scope.CFG = CFG = null
    $http.get('/api/redstone/config').success((data) ->
        if typeof(data) == "string"
            window.alert(data)
            return
        $scope.CFG = CFG = data
        CFG.sourcekeys = _.keys(CFG.sources)
        $scope.addsource = 'longranger'

        $scope.redstone = {
            from:       '',
            to:         '',
            desc:       '',
            dtl:        CFG.defaults.dtl,
            dlmax:      CFG.defaults.dlmax,
            cost_est:   0,
            size_est:   0,
            samples: []
        }
    )

    $scope.newSample = (data) ->
        files = {}
        source = CFG.sources[data.source]
        for f in _.keys(data.fileinfo)
            files[f] = {
                include: data.fileinfo[f].include,
                path: data.fileinfo[f].path,
                size: data.fileinfo[f].size,
            }
        if data.source != "folder"
            if $scope.redstone.samples.length > 0
                lastSample = $scope.redstone.samples[$scope.redstone.samples.length-1]
                for f in _.keys(files)
                    if lastSample.files[f]?
                        files[f].include = lastSample.files[f].include

        # Translate spaces to underscores, and remove non-alphanum
        if data.idtype == 'lena'
            name = data.bag.description
        else if data.idtype == 'path'
            name = data.id.split("/").reverse()[0]
        name = name.replace(///\s+///g, '_').replace(///[^\d\w]+///g, '')
        
        $scope.redstone.samples.push({
            source:     source,
            sourcename: data.source,
            container:  data.container,
            id:         data.id,
            idtype:     data.idtype,
            versions:   data.versions.reverse(),
            version:    data.versions[0],
            name:       name,
            files:      files,
            sizetotal:  0,
            hsize:      '',
            cost:       '',
        })

    $scope.validate = () ->
        reqsamps = []
        samps = $scope.redstone.samples
        totalsize = 0.0
        totalcost = 0.0
        for s in samps
            s.sizetotal = 0
            for f in _.keys(s.files)
                if s.files[f].include
                    s.sizetotal += s.files[f].size
            totalsize += s.sizetotal
            s.hsize = Humanize.fileSize(s.sizetotal)
            gb = s.sizetotal / (1024 * 1024 * 1024)
            storage_cost = gb * CFG.prices.s3_storage_per_gbmo * ($scope.redstone.dtl / 30)
            download_cost = gb * CFG.prices.s3_download_per_gb * $scope.redstone.dlmax
            totalcost += storage_cost + download_cost
            s.cost = Humanize.formatNumber(storage_cost + download_cost, 2)
            sfiles = []
            if s.sourcename != "folder"
                for f in s.source.order
                    if s.files[f].include
                        sfiles.push(f)
            reqsamps.push([ s.idtype, s.id, s.container, s.version, s.name, sfiles.join('|') ].join(','))
        $scope.redstone.totalsize = Humanize.fileSize(totalsize)
        $scope.redstone.totalcost = '$' + Humanize.formatNumber(totalcost, 2)
        desc = $scope.redstone.desc
        desc = desc.replace(///\s+///g, '_')
        desc = desc.replace(///[^\d\w]+///g, '')
        request = {
            date:       moment().format(),
            from:       $scope.redstone.from,
            to:         $scope.redstone.to,
            desc:       desc,
            dtl:        $scope.redstone.dtl,
            dlmax:      $scope.redstone.dlmax,
            totalsize:  $scope.redstone.totalsize,
            totalcost:  $scope.redstone.totalcost,
            samples:    reqsamps,
        }
        $scope.output = angular.toJson(request, 4)

    $scope.addSample = () ->
        source = CFG.sources[$scope.addsource]
        idtype = if $scope.addid[0] == '/' then 'path' else 'lena'
        params = {
            source: $scope.addsource,
            type: source.type,
            id: $scope.addid,
            idtype: idtype,
            pname: source.pname,
            paths: source.paths,
        }
        $http.post('/api/redstone/validate', params).success((data) ->
            if typeof(data) == "string"
                window.alert(data)
                return
            $scope.newSample(data)
            $scope.validate()
        )
        # Auto-increment if LENA id
        if idtype == 'lena'
            $scope.addid = '' + (parseInt($scope.addid) + 1)
        else
            $scope.addid = ''

    $scope.close = (i) ->
        $scope.redstone.samples.splice(i, 1)
        $scope.validate()
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
