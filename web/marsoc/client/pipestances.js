(function() {
  var app, callApiWithConfirmation;

  app = angular.module('app', ['ui.bootstrap']);

  callApiWithConfirmation = function($scope, $http, $p, $url) {
    var psid;
    $scope.showbutton = false;
    psid = window.prompt("Please type the sample ID to confirm");
    if (psid === $p.psid) {
      return $http.post($url, {
        fcid: $p.fcid,
        pipeline: $p.pipeline,
        psid: $p.psid
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
        $scope.pipestances = _.sortBy(data, function(p) {
          return [p.fcid, p.pipeline, p.psid, p.state];
        });
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
    $scope.filterPipestance = function(p) {
      var prop, _i, _len, _ref;
      _ref = ['fcid', 'pipeline', 'psid', 'state'];
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        prop = _ref[_i];
        if ($scope[prop] && $scope[prop] !== p[prop]) {
          return false;
        }
      }
      return true;
    };
    $scope.archivePipestance = function(p) {
      return callApiWithConfirmation($scope, $http, p, '/api/archive-sample');
    };
    $scope.wipePipestance = function(p) {
      return callApiWithConfirmation($scope, $http, p, '/api/wipe-sample');
    };
    $scope.killPipestance = function(p) {
      return callApiWithConfirmation($scope, $http, p, '/api/kill-sample');
    };
    $scope.unfailPipestance = function(p) {
      return callApiWithConfirmation($scope, $http, p, '/api/restart-sample');
    };
    $scope.capitalize = function(str) {
      return str[0].toUpperCase() + str.slice(1);
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
