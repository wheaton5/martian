(function() {
  var app, callApi, getTests;

  app = angular.module('app', ['ui.bootstrap']);

  callApi = function($scope, $http, $data, $url) {
    $scope.showbutton = false;
    return $http.post($url, $data).success(function(data) {
      $scope.refreshProgram();
      if (data) {
        return window.alert(data.toString());
      }
    });
  };

  getTests = function(tests, state) {
    var i, len, results, test;
    results = [];
    for (i = 0, len = tests.length; i < len; i++) {
      test = tests[i];
      if (test.state === state) {
        results.push(test);
      }
    }
    return results;
  };

  app.controller('ProgramCtrl', function($scope, $http, $interval, $modal) {
    $scope.admin = admin;
    $scope.program_name = program_name;
    $scope.cycle_id = cycle_id;
    $scope.showbutton = true;
    $scope.program = null;
    $scope.cycle = null;
    $scope.packages = null;
    $scope.refreshProgram = function() {
      $http.get('/api/program/' + $scope.program_name + '/' + $scope.cycle_id).success(function(data) {
        $scope.program = data;
        $scope.cycle = data.cycles[0];
        return $scope.showbutton = true;
      });
      return $http.get('/api/manage/get-items').success(function(data) {
        var p;
        return $scope.packages = (function() {
          var i, len, ref, results;
          ref = data.packages;
          results = [];
          for (i = 0, len = ref.length; i < len; i++) {
            p = ref[i];
            if (p.state === 'complete') {
              results.push(p);
            }
          }
          return results;
        })();
      });
    };
    $scope.refreshProgram();
    $scope.isCycleActive = function() {
      if ($scope.cycle) {
        return $scope.cycle.end_date.length === 0;
      }
      return false;
    };
    $scope.endCycle = function() {
      var data, value;
      $scope.showbutton = false;
      value = window.confirm('Are you sure you want to end this cycle?');
      if (value) {
        data = {
          program_name: $scope.program.name,
          cycle_id: $scope.cycle.id
        };
        return callApi($scope, $http, data, '/api/program/end-cycle');
      } else {
        window.alert('This cycle is still active!');
        return $scope.showbutton = true;
      }
    };
    $scope.someTests = function(round, state) {
      var tests;
      tests = getTests(round.tests, state);
      return tests.length > 0;
    };
    $scope.invoke = function(round) {
      return $scope.pipestancesForm(round, 'Invoke Pipestances', 'ready', '/api/test/invoke-pipestances');
    };
    $scope.unfail = function(round) {
      return $scope.pipestancesForm(round, 'Unfail pipestances', 'failed', '/api/test/restart-pipestances');
    };
    $scope.kill = function(round) {
      return $scope.pipestancesForm(round, 'Kill Pipestances', 'running', '/api/test/kill-pipestances');
    };
    $scope.wipe = function(round) {
      return $scope.pipestancesForm(round, 'Wipe Pipestances', 'failed', '/api/test/wipe-pipestances');
    };
    $scope.pipestancesForm = function(round, title, state, url) {
      var modalInstance;
      modalInstance = $modal.open({
        animation: true,
        templateUrl: 'pipestances.html',
        controller: 'PipestancesCtrl',
        resolve: {
          tests: function() {
            return getTests(round.tests, state);
          },
          title: function() {
            return title;
          },
          url: function() {
            return url;
          }
        }
      });
      return modalInstance.result.then(function(data) {
        var test, tests;
        tests = (function() {
          var i, len, ref, results;
          ref = data.tests;
          results = [];
          for (i = 0, len = ref.length; i < len; i++) {
            test = ref[i];
            if (test.selected) {
              results.push(test);
            }
          }
          return results;
        })();
        return callApi($scope, $http, tests, data.url);
      }, null);
    };
    $scope.startRoundForm = function() {
      var modalInstance;
      modalInstance = $modal.open({
        animation: true,
        templateUrl: 'start_round.html',
        controller: 'StartRoundCtrl',
        resolve: {
          program_name: function() {
            return $scope.program.name;
          },
          cycle_id: function() {
            return $scope.cycle.id;
          },
          round_id: function() {
            return $scope.cycle.rounds.length + 1;
          },
          packages: function() {
            return $scope.packages;
          }
        }
      });
      return modalInstance.result.then(function(data) {
        return callApi($scope, $http, data, '/api/cycle/start-round');
      }, null);
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshProgram();
      }), 5000);
    }
  });

  app.controller('StartRoundCtrl', function($scope, $modalInstance, program_name, cycle_id, round_id, packages) {
    $scope.data = {
      program_name: program_name,
      cycle_id: cycle_id,
      round_id: round_id
    };
    $scope.packages = packages;
    $scope.formatPackage = function(p) {
      return p.name + ' : ' + p.target + ' : ' + p.mro_version + ' : ' + p.build_date;
    };
    $scope.startRound = function() {
      return $modalInstance.close($scope.data);
    };
    return $scope.cancelRound = function() {
      return $modalInstance.dismiss('cancel');
    };
  });

  app.controller('PipestancesCtrl', function($scope, $modalInstance, tests, title, url) {
    $scope.tests = tests;
    $scope.title = title;
    $scope.url = url;
    $scope.selectAll = function() {
      var i, len, ref, results, test;
      ref = $scope.tests;
      results = [];
      for (i = 0, len = ref.length; i < len; i++) {
        test = ref[i];
        results.push(test.selected = !test.selected);
      }
      return results;
    };
    $scope.startPipestances = function() {
      var data;
      data = {
        tests: $scope.tests,
        url: $scope.url
      };
      return $modalInstance.close(data);
    };
    return $scope.cancelPipestances = function() {
      return $modalInstance.dismiss('cancel');
    };
  });

}).call(this);
