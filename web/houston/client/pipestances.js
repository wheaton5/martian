(function() {
  var app, callApi, callApiWithConfirmation;

  app = angular.module('app', ['ui.bootstrap']);

  callApiWithConfirmation = function($scope, $http, $p, $url) {
    var psid;
    $scope.showbutton = false;
    psid = window.prompt("Please type the sample ID to confirm");
    if (psid === $p.psid) {
      return callApi($scope, $http, $p, $url);
    } else {
      window.alert("Incorrect sample ID");
      return $scope.showbutton = true;
    }
  };

  callApi = function($scope, $http, $p, $url) {
    $scope.showbutton = false;
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
  };

  app.controller('PipestancesCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.state = state;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.autoinvoke = {
      button: true,
      state: false
    };
    $scope.showbutton = true;
    $scope.name = null;
    $scope.fcid = null;
    $scope.pipeline = null;
    $scope.psid = null;
    $scope.refreshPipestances = function() {
      $http.get('/api/get-pipestances').success(function(data) {
        var fcids, names, p, pipelines, psids, _i, _len, _ref;
        $scope.pipestances = _.sortBy(data, function(p) {
          return [p.fcid, p.pipeline, p.psid, p.state];
        });
        names = {};
        fcids = {};
        pipelines = {};
        psids = {};
        _ref = $scope.pipestances;
        for (_i = 0, _len = _ref.length; _i < _len; _i++) {
          p = _ref[_i];
          names[p.name] = true;
          fcids[p.fcid] = true;
          pipelines[p.pipeline] = true;
          psids[p.psid] = true;
        }
        $scope.names = _.keys(names);
        $scope.fcids = _.keys(fcids);
        $scope.pipelines = _.keys(pipelines);
        $scope.psids = _.keys(psids);
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
      _ref = ['name', 'fcid', 'pipeline', 'psid', 'state'];
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        prop = _ref[_i];
        if ($scope[prop] && $scope[prop] !== p[prop]) {
          return false;
        }
      }
      return true;
    };
    $scope.invokePipestance = function(p) {
      return callApi($scope, $http, p, '/api/invoke-sample');
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
      return callApi($scope, $http, p, '/api/restart-sample');
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
