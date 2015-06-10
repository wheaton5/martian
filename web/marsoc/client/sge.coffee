#
# Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
#
# Angular controllers for marsoc qweb main UI.
#

app = angular.module('app', ['ui.bootstrap'])

allq = (x) ->
    return x.Name.indexOf("all.q") == 0

app.controller('QWebCtrl', ($scope, $http, $interval) ->
    $http.get('/api/qstat').success((data) ->
        if typeof(data) == "string"
            window.alert(data)
            return

        console.log(data)

        qlist = data.Queue_info[0].Queue_list
        console.log(qlist)
        $scope.slots_total = _.sum(_.pluck(_.filter(qlist, allq), "Slots_total"))
        $scope.slots_used = _.sum(_.pluck(qlist, "Slots_used"))
        # Populate queue names into the jobs
        for q in qlist
            if q.Job_list
                for job in q.Job_list
                    job.Queue_name = q.Name

        $scope.jobs = _.sortBy(_.flatten(_.compact(_.pluck(qlist, "Job_list"))), "JB_name")
        $scope.running_count = $scope.jobs.length

        jlist = data.Job_info[0].Job_list
        console.log(jlist)

        $scope.pending_count = 0
        $scope.error_count = 0
        
        if jlist
            jobs = _.sortBy(jlist, "JB_name")
            pjobs = _.pluck(pjobs, { "State": "pending" })
            $scope.pending_count = pjobs.length
            ejobs = _.pluck(pjobs, { "StateCode": "E" })
            $scope.error_count = ejobs.length
            console.log(pjobs)
            $scope.jobs = _.flatten([ $scope.jobs, pjobs, ejobs ])

    )
)
