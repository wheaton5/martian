(function() {
  var allq, app;

  app = angular.module('app', ['ui.bootstrap']);

  allq = function(x) {
    return x.Name.indexOf("all.q") === 0;
  };

  app.controller('QWebCtrl', function($scope, $http, $interval) {
    return $http.get('/api/qstat').success(function(data) {
      var ejobs, f, j, jlist, job, jobs, k, lb, parts, pjobs, q, qlist, rjobs, slots, t, v, _i, _j, _k, _l, _len, _len1, _len2, _len3, _len4, _len5, _len6, _len7, _m, _n, _o, _p, _ref, _ref1, _ref2, _ref3, _ref4, _ref5, _results;
      if (typeof data === "string") {
        window.alert(data);
        return;
      }
      qlist = data.Queue_info[0].Queue_list;
      $scope.slots_total = _.sum(_.pluck(_.filter(qlist, allq), "Slots_total"));
      $scope.slots_used = _.sum(_.pluck(qlist, "Slots_used"));
      for (_i = 0, _len = qlist.length; _i < _len; _i++) {
        q = qlist[_i];
        if (q.Job_list) {
          _ref = q.Job_list;
          for (_j = 0, _len1 = _ref.length; _j < _len1; _j++) {
            job = _ref[_j];
            job.Queue_name = q.Name;
          }
        }
      }
      $scope.jobs = _.sortBy(_.compact(_.flatten(_.pluck(qlist, "Job_list"))), "JB_name");
      slots = {};
      _ref1 = $scope.jobs;
      for (_k = 0, _len2 = _ref1.length; _k < _len2; _k++) {
        j = _ref1[_k];
        if (j.JB_job_number in slots) {
          slots[j.JB_job_number] += j.Slots;
        } else {
          slots[j.JB_job_number] = j.Slots;
        }
      }
      _ref2 = $scope.jobs;
      for (_l = 0, _len3 = _ref2.length; _l < _len3; _l++) {
        j = _ref2[_l];
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
          "State": "pending"
        });
        $scope.pending_count = pjobs.length;
        ejobs = _.where(jobs, {
          "StateCode": "E"
        });
        $scope.error_count = ejobs.length;
        $scope.jobs = _.compact(_.flattenDeep([$scope.jobs, pjobs, ejobs]));
      }
      _ref3 = $scope.jobs;
      for (_m = 0, _len4 = _ref3.length; _m < _len4; _m++) {
        j = _ref3[_m];
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
        _ref4 = ["JAT_start_time", "JB_submission_time"];
        for (_n = 0, _len5 = _ref4.length; _n < _len5; _n++) {
          f = _ref4[_n];
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
      _ref5 = ["stage", "JB_owner", "psid", "JB_job_number"];
      _results = [];
      for (_o = 0, _len6 = _ref5.length; _o < _len6; _o++) {
        t = _ref5[_o];
        lb = {};
        for (_p = 0, _len7 = rjobs.length; _p < _len7; _p++) {
          j = rjobs[_p];
          k = j[t];
          v = t === "JB_job_number" ? j.mins : j.Slots;
          lb[k] = k in lb ? lb[k] + v : v;
        }
        _results.push($scope.lboards[t] = _.take(_.sortBy(_.map(lb, function(v, k) {
          return {
            item: k,
            count: v
          };
        }), "count").reverse(), 5));
      }
      return _results;
    });
  });

}).call(this);
