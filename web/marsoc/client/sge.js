(function() {
  var allq, app;

  app = angular.module('app', ['ui.bootstrap']);

  allq = function(x) {
    return x.Name.indexOf("all.q") === 0;
  };

  app.controller('QWebCtrl', function($scope, $http, $interval) {
    return $http.get('/api/qstat').success(function(data) {
      var ejobs, f, i, j, jlist, job, jobs, k, l, lb, len, len1, len2, len3, len4, len5, len6, len7, m, n, o, p, parts, pjobs, q, qlist, r, ref, ref1, ref2, ref3, ref4, ref5, results, rjobs, s, slots, t, v;
      if (typeof data === "string") {
        window.alert(data);
        return;
      }
      qlist = data.Queue_info[0].Queue_list;
      $scope.slots_total = _.sum(_.pluck(_.filter(qlist, allq), "Slots_total"));
      $scope.slots_used = _.sum(_.pluck(qlist, "Slots_used"));
      for (i = 0, len = qlist.length; i < len; i++) {
        q = qlist[i];
        if (q.Job_list) {
          ref = q.Job_list;
          for (l = 0, len1 = ref.length; l < len1; l++) {
            job = ref[l];
            job.Queue_name = q.Name;
          }
        }
      }
      $scope.jobs = _.sortBy(_.compact(_.flatten(_.pluck(qlist, "Job_list"))), "JB_name");
      slots = {};
      ref1 = $scope.jobs;
      for (m = 0, len2 = ref1.length; m < len2; m++) {
        j = ref1[m];
        if (j.JB_job_number in slots) {
          slots[j.JB_job_number] += j.Slots;
        } else {
          slots[j.JB_job_number] = j.Slots;
        }
      }
      ref2 = $scope.jobs;
      for (n = 0, len3 = ref2.length; n < len3; n++) {
        j = ref2[n];
        j.Slots = slots[j.JB_job_number];
      }
      $scope.jobs = _.uniq($scope.jobs, "JB_job_number");
      $scope.running_count = $scope.jobs.length;
      $scope.pending_count = 0;
      $scope.error_count = 0;
      jlist = data.Job_info[0].Job_list;
      if (jlist) {
        jobs = _.sortBy(jlist, "JB_name");
        pjobs = _.where(jobs, {
          "StateCode": "qw"
        });
        $scope.pending_count = pjobs.length;
        ejobs = _.where(jobs, {
          "StateCode": "Eqw"
        });
        $scope.error_count = ejobs.length;
        $scope.jobs = _.compact(_.flattenDeep([$scope.jobs, pjobs, ejobs]));
      }
      ref3 = $scope.jobs;
      for (o = 0, len4 = ref3.length; o < len4; o++) {
        j = ref3[o];
        j.JB_owner = j.JB_owner === "mario" ? "marsoc" : j.JB_owner;
        parts = j.JB_name.split(".");
        j.psid = parts[1];
        if (parts.slice(-1)[0] === "main") {
          j.stage = parts.slice(-4, -3)[0];
          j.chunk = parts.slice(-3).join(".");
        } else {
          j.stage = parts.slice(-3, -2)[0];
          j.chunk = parts.slice(-2).join(".");
        }
        j.node = j.Queue_name.split("@")[1];
        ref4 = ["JAT_start_time", "JB_submission_time"];
        for (p = 0, len5 = ref4.length; p < len5; p++) {
          f = ref4[p];
          if (j[f]) {
            j.time = moment(j[f]);
            j.mins = moment().diff(j.time, "minutes");
            j.ago = j.time.fromNow(true);
          }
        }
      }
      $scope.lboards = {};
      rjobs = _.where($scope.jobs, {
        "State": "running"
      });
      ref5 = ["stage", "JB_owner", "psid", "JB_job_number"];
      results = [];
      for (r = 0, len6 = ref5.length; r < len6; r++) {
        t = ref5[r];
        lb = {};
        for (s = 0, len7 = rjobs.length; s < len7; s++) {
          j = rjobs[s];
          k = j[t];
          v = t === "JB_job_number" ? j.mins : j.Slots;
          lb[k] = k in lb ? lb[k] + v : v;
        }
        results.push($scope.lboards[t] = _.take(_.sortBy(_.map(lb, function(v, k) {
          return {
            item: k,
            count: v
          };
        }), "count").reverse(), 5));
      }
      return results;
    });
  });

}).call(this);
