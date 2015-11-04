#
# Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
#
# Angular controllers for Redstone main UI.
#

app = angular.module('app', ['ui.bootstrap'])

app.controller('RedstoneCtrl', ($scope, $http, $interval) ->
    # Fetch configuration from the server
    $scope.CFG = CFG = null
    $http.get('/api/redstone/config').success((data) ->
        if typeof(data) == "string"
            window.alert(data)
            return
        $scope.CFG = CFG = data
        CFG.sourcekeys = _.keys(CFG.sources)

        # Set up initial state
        $scope.addsource = 'longranger'
        $scope.redstone = {
            from:       '',
            to:         '',
            desc:       '',
            dtl:        CFG.defaults.dtl,
            dlmax:      CFG.defaults.dlmax,
            cost_est:   0,
            size_est:   0,
            bundles:    []
        }
    )

    # Handle user request to add a bundle
    $scope.addBundle = () ->
        # Get source and identifier to be added from the UI
        sname = $scope.addsource
        id = $scope.addid

        # Look up the source data
        source = CFG.sources[sname]
        stype = source.type

        # Detect type of id
        if stype == 'folder'
            itype = 'path'
        else
            itype = if id[0] == '/' then 'path' else 'lena'

        params = {
            sname: sname,
            stype: stype,
            id:    id,
            itype: itype,
            pname: source.pname,
            paths: source.paths,
        }
        $http.post('/api/redstone/validate', params).success((data) ->
            if typeof(data) == "string"
                window.alert(data)
                return
            console.log(data)
            $scope.makeBundle(data)
            $scope.refresh()

            # Auto-increment if LENA id
            if itype == 'lena'
                $scope.addid = '' + (parseInt($scope.addid) + 1)
            else
                $scope.addid = ''
        )

    $scope.makeBundle = (data) ->
        stype = data.stype

        if stype == 'folder'
            # Name is just last path element
            # Translate spaces to underscores, and remove non-alphanum
            name = data.id.split("/").reverse()[0]
            name = name.replace(///\s+///g, '_').replace(///[^\d\w]+///g, '')
            $scope.redstone.bundles.push({
                stype:  stype,
                id:     data.id,
                itype:  'path',
                name:   name,
                files:  data.files,
                fcount: _.keys(data.files).length,
            })

        else if stype == 'pipestance'
            source = CFG.sources[data.sname]

            # Copy file selections from previous bundle
            if $scope.redstone.bundles.length > 0
                lastBundle = $scope.redstone.bundles[$scope.redstone.bundles.length-1]
                for f in _.keys(data.files)
                    if lastBundle.files[f]?
                        data.files[f].include = lastBundle.files[f].include

            # Translate spaces to underscores, and remove non-alphanum
            if data.itype == 'lena'
                name = data.bag.description
            else if data.itype == 'path'
                name = data.id.split("/").reverse()[0]
            name = name.replace(///\s+///g, '_').replace(///[^\d\w]+///g, '')
        
            $scope.redstone.bundles.push({
                stype:      stype,
                sname:      data.sname,
                source:     source,
                container:  data.container,
                id:         data.id,
                itype:      data.itype,
                versions:   data.versions.reverse(),
                version:    data.versions[0],
                name:       "FILL THIS IN",
                files:      data.files,
            })

    $scope.refresh = () ->
        bundledeets = []
        totalsize = 0.0
        totalcost = 0.0
        for b in $scope.redstone.bundles
            # Add up the size and cost of all the files for the bundle
            b.bsize = 0
            for f in _.keys(b.files)
                if b.files[f].include
                    b.bsize += b.files[f].size
            totalsize += b.bsize
            b.hsize = Humanize.fileSize(b.bsize)
            gb = b.bsize / (1024 * 1024 * 1024)
            storage_cost = gb * CFG.prices.s3_storage_per_gbmo * ($scope.redstone.dtl / 30)
            download_cost = gb * CFG.prices.s3_download_per_gb * $scope.redstone.dlmax
            totalcost += storage_cost + download_cost
            b.cost = Humanize.formatNumber(storage_cost + download_cost, 2)
            bfiles = []

            # For pipestances, add the encoded file names
            if b.stype == "pipestance"
                for f in b.source.order
                    if b.files[f].include
                        bfiles.push(f)

            b.name = b.name.replace(///\s+///g, '_').replace(///[^\d\w]+///g, '')
            bundledeets.push([ b.stype, b.sname, b.itype, b.id, b.container, b.version, b.name, bfiles.join('|') ].join(','))

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
            bundles:    bundledeets,
        }
        $scope.output = angular.toJson(request, 4)

    $scope.close = (i) ->
        $scope.redstone.bundles.splice(i, 1)
        $scope.refresh()
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
