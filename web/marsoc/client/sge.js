(function() {
  var allq, app;

  app = angular.module('app', ['ui.bootstrap']);

  allq = function(x) {
    return x.Name.indexOf("all.q") === 0;
  };

  app.controller('QWebCtrl', function($scope, $http, $interval) {
    return $http.get('/api/qstat').success(function(data) {
      var ejobs, jlist, job, jobs, pjobs, q, qlist, _i, _j, _len, _len1, _ref;
      if (typeof data === "string") {
        window.alert(data);
        return;
      }
      console.log(data);
      qlist = data.Queue_info[0].Queue_list;
      console.log(qlist);
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
      $scope.jobs = _.sortBy(_.flatten(_.compact(_.pluck(qlist, "Job_list"))), "JB_name");
      $scope.running_count = $scope.jobs.length;
      jlist = data.Job_info[0].Job_list;
      console.log(jlist);
      $scope.pending_count = 0;
      $scope.error_count = 0;
      if (jlist) {
        jobs = _.sortBy(jlist, "JB_name");
        pjobs = _.pluck(pjobs, {
          "State": "pending"
        });
        $scope.pending_count = pjobs.length;
        ejobs = _.pluck(pjobs, {
          "StateCode": "E"
        });
        $scope.error_count = ejobs.length;
        console.log(pjobs);
        return $scope.jobs = _.flatten([$scope.jobs, pjobs, ejobs]);
      }
    });
  });

}).call(this);
