(function() {
  var app, callApiWithConfirmation;

  app = angular.module('app', ['ui.bootstrap']);

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

  app.controller('PipestancesCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.autoinvoke = {
      button: true,
      state: false
    };
    $scope.showbutton = true;
    $scope.fcid = null;
    $scope.pipeline = null;
    $scope.psid = null;
    $scope.state = "running";
    $scope.refreshPipestances = function() {
      $http.get('/api/get-pipestances').success(function(data) {
        var fcids, p, pipelines, psids, _i, _len, _ref;
        $scope.pipestances = data;
        fcids = [];
        pipelines = [];
        psids = [];
        _ref = $scope.pipestances;
        for (_i = 0, _len = _ref.length; _i < _len; _i++) {
          p = _ref[_i];
          fcids.push(p.fcid);
          pipelines.push(p.pipeline);
          psids.push(p.psid);
        }
        $scope.fcids = _.uniq(fcids);
        $scope.pipelines = _.uniq(pipelines);
        $scope.psids = _.uniq(psids);
        return $scope.showbutton = true;
      });
      return $http.get('/api/get-auto-invoke-status').success(function(data) {
        $scope.autoinvoke.state = data.state;
        return $scope.autoinvoke.button = true;
      });
    };
    $scope.refreshPipestances();
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
    $scope.getAutoInvokeClass = function() {};
    if ($scope.autoinvoke.state) {
      return "complete";
    } else {
      return "failed";
    }
    $scope.setAutoInvoke = function() {
      $scope.autoinvoke.button = false;
      return $http.post('/api/set-auto-invoke-status', {
        state: !$scope.autoinvoke.state
      }).success(function(data) {
        $scope.refreshPipestances();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshPipestances();
      }), 5000);
    }
  });

}).call(this);
