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
    $scope.invokeAll = function(round) {
      var tests;
      tests = getTests(round.tests, 'ready');
      return callApi($scope, $http, tests, '/api/test/invoke-pipestances');
    };
    $scope.unfailAll = function(round) {
      var tests;
      tests = getTests(round.tests, 'failed');
      return callApi($scope, $http, tests, '/api/test/restart-pipestances');
    };
    $scope.killAll = function(round) {
      var tests;
      tests = getTests(round.tests, 'running');
      return callApi($scope, $http, tests, '/api/test/kill-pipestances');
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

}).call(this);
