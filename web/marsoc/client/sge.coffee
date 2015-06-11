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
        # String result means error, JSON object on success
        if typeof(data) == "string"
            window.alert(data)
            return

        # Extract the queue_list, which is a list of host-queues
        qlist = data.Queue_info[0].Queue_list
        $scope.slots_total = _.sum(_.pluck(_.filter(qlist, allq), "Slots_total"))
        $scope.slots_used = _.sum(_.pluck(qlist, "Slots_used"))

        # Populate queue names into the jobs in each host-queue
        for q in qlist
            if q.Job_list
                for job in q.Job_list
                    job.Queue_name = q.Name

        # Extract a flat list of jobs from queue list, and
        # sort by FQ job name
        $scope.jobs = _.sortBy(_.compact(_.flatten(_.pluck(qlist, "Job_list"))), "JB_name")

        # SGE seems to be splitting single multi-thread jobs
        # like 4-way bwa into multiple queues. So merge them
        # back together as a single job with summed slots.
        slots = {}
        for j in $scope.jobs
            if j.JB_job_number of slots
                slots[j.JB_job_number] += j.Slots
            else
                slots[j.JB_job_number] = j.Slots
        for j in $scope.jobs
            j.Slots = slots[j.JB_job_number]
        $scope.jobs = _.uniq($scope.jobs, "JB_job_number")
        $scope.running_count = $scope.jobs.length

        # Extract list of non-running jobs
        $scope.pending_count = 0
        $scope.error_count = 0
        jlist = data.Job_info[0].Job_list
        if jlist
            # Sort jobs by name
            jobs = _.sortBy(jlist, "JB_name")

            # Count pending and errored jobs
            pjobs = _.where(jobs, { "StateCode": "qw" })
            $scope.pending_count = pjobs.length
            ejobs = _.where(jobs, { "StateCode": "Eqw" })
            $scope.error_count = ejobs.length

            # Append pending and error jobs to running jobs
            $scope.jobs = _.compact(_.flattenDeep([ $scope.jobs, pjobs, ejobs ]))

        # Post-process fields for display
        for j in $scope.jobs
            j.JB_owner = if j.JB_owner == "mario" then "marsoc" else j.JB_owner
            parts = j.JB_name.split(".")
            j.psid = parts[1]
            if parts[-1..-1][0] == "main"
                j.stage = parts[-4..-4][0]
                j.chunk = parts[-3..].join(".")
            else
                j.stage = parts[-3..-3][0]
                j.chunk = parts[-2..].join(".")
            j.node = j.Queue_name.split("@")[1]

            for f in [ "JAT_start_time", "JB_submission_time" ]
                if j[f]
                    j.time = moment(j[f])
                    j.mins = moment().diff(j.time, "minutes")
                    j.ago = j.time.fromNow(true)

        # Leaderboards
        $scope.lboards = {}
        rjobs = _.where($scope.jobs, { "State": "running" })
        for t in [ "stage", "JB_owner", "psid", "JB_job_number" ]
            lb = {}
            for j in rjobs
                k = j[t]
                v = if t == "JB_job_number" then j.mins else j.Slots
                lb[k] = if k of lb then lb[k] + v else v

            # Convert countmap to array of maps, sort by count,
            # largest first, then cap at 6.
            $scope.lboards[t] = _.take(_.sortBy(_.map(lb, (v, k) ->
                return { item: k, count: v }
            ), "count").reverse(), 5)
    )
)
