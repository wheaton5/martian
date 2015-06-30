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
    $scope.refreshProgram = function() {
      $http.get('/api/program/' + $scope.program_name + '/' + $scope.cycle_id.toString()).success(function(data) {
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
    $scope.someTests = function(round, state) {
      var tests;
      tests = getTests(round.tests, state);
      return tests.length > 0;
    };
    $scope.invoke = function(round) {
      return $scope.pipestancesForm(round, 'Invoke Pipestances', 'ready');
    };
    $scope.unfail = function(round) {
      return $scope.pipestancesForm(round, 'Unfail pipestances', 'failed');
    };
    $scope.kill = function(round) {
      return $scope.pipestancesForm(round, 'Kill Pipestances', 'running');
    };
    $scope.pipestancesForm = function(round, title, state) {
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
          state: function() {
            return state;
          }
        }
      });
      return modalInstance.result.then(function(data) {
        var test, tests, url;
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
        switch (data.state) {
          case 'ready':
            url = '/api/test/invoke-pipestances';
            break;
          case 'failed':
            url = '/api/test/restart-pipestances';
            break;
          case 'running':
            url = '/api/test/kill-pipestances';
        }
        return callApi($scope, $http, tests, url);
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
      return p.name + ' : ' + p.target + ' : ' + p.mro_version;
    };
    $scope.startRound = function() {
      return $modalInstance.close($scope.data);
    };
    return $scope.cancelRound = function() {
      return $modalInstance.dismiss('cancel');
    };
  });

  app.controller('PipestancesCtrl', function($scope, $modalInstance, tests, title, state) {
    $scope.tests = tests;
    $scope.title = title;
    $scope.state = state;
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
        state: $scope.state
      };
      return $modalInstance.close(data);
    };
    return $scope.cancelPipestances = function() {
      return $modalInstance.dismiss('cancel');
    };
  });

}).call(this);
