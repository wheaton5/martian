(function() {
  var app, callApiWithConfirmation;

  callApiWithConfirmation = function($scope, $http, $url) {
    var psid;
    $scope.showbutton = false;
    psid = window.prompt("Please type the sample ID to confirm");
    if (psid === $scope.selps.psid) {
      return $http.post($url, {
        psid: $scope.selps.psid
      }).success(function(data) {
        $scope.refreshPipestances();
        if (data) {
          return window.alert(data.toString());
        }
      });
    } else {
      return window.alert("Incorrect sample ID");
    }
  };

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('PipestancesCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.showbutton = true;
    $scope.fcid = null;
    $scope.pipeline = null;
    $scope.psid = null;
    $scope.state = "running";
    $scope.refreshPipestances();
    $scope.refreshPipestances = function() {
      return $http.get('/api/get-pipestances').success(function(data) {
        var p, _i, _len, _ref;
        $scope.pipestances = data;
        _ref = $scope.pipestances;
        for (_i = 0, _len = _ref.length; _i < _len; _i++) {
          p = _ref[_i];
          $scope.fcids.push(p.fcid);
          $scope.pipelines.push(p.pipeline);
          $scope.psids.push(p.psid);
        }
        $scope.fcids = _.uniq(fcids);
        $scope.pipelines = _.uniq(pipelines);
        $scope.psids = _.uniq(psids);
        return $scope.showbutton = true;
      });
    };
    $scope.archivePipestance = function() {
      return callApiWithConfirmation($scope, $http, '/api/archive-sample');
    };
    $scope.wipePipestance = function() {
      return callApiWithConfirmation($scope, $http, '/api/wipe-sample');
    };
    $scope.killPipestance = function() {
      return callApiWithConfirmation($scope, $http, '/api/kill-sample');
    };
    $scope.unfailPipestance = function() {
      return callApiWithConfirmation($scope, $http, '/api/unfail-sample');
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshPipestances();
      }), 5000);
    }
  });

}).call(this);
