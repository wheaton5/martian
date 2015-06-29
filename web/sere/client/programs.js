(function() {
  var app, callApi;

  app = angular.module('app', ['ui.bootstrap']);

  callApi = function($scope, $http, $data, $url) {
    return $http.post($url, $data).success(function(data) {
      $scope.refreshPrograms();
      if (data) {
        return window.alert(data.toString());
      }
    });
  };

  app.controller('ProgramsCtrl', function($scope, $http, $interval, $modal) {
    $scope.admin = admin;
    $scope.programs = null;
    $scope.refreshPrograms = function() {
      return $http.get('/api/program/get-programs').success(function(data) {
        return $scope.programs = data;
      });
    };
    $scope.refreshPrograms();
    $scope.isProgramActive = function(program) {
      var cycle;
      if (program.cycles.length > 0) {
        cycle = program.cycles[program.cycles.length - 1];
        return (cycle.end_date != null) || cycle.end_date.length === 0;
      }
      return false;
    };
    $scope.startCycleForm = function(program) {
      var modalInstance;
      modalInstance = $modal.open({
        animation: true,
        templateUrl: 'start_cycle.html',
        controller: 'StartCycleCtrl',
        resolve: {
          program_name: function() {
            return program.name;
          },
          cycle_id: function() {
            return program.cycles.length + 1;
          }
        }
      });
      return modalInstance.result.then(function(data) {
        return callApi($scope, $http, data, '/api/program/start-cycle');
      }, null);
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshPrograms();
      }), 5000);
    }
  });

  app.controller('StartCycleCtrl', function($scope, $modalInstance, program_name, cycle_id) {
    $scope.data = {
      program_name: program_name,
      cycle_id: cycle_id
    };
    $scope.startCycle = function() {
      return $modalInstance.close($scope.data);
    };
    return $scope.cancelCycle = function() {
      return $modalInstance.dismiss('cancel');
    };
  });

}).call(this);
