(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.filter('momentFormat', function() {
    return function(time, fmt) {
      return moment(time).format(fmt);
    };
  }).filter('momentTimeAgo', function() {
    return function(time) {
      return moment(time).fromNow();
    };
  }).filter('flowcellFront', function() {
    return function(fcid) {
      return fcid.substr(0, 5);
    };
  }).filter('flowcellBack', function() {
    return function(fcid) {
      return fcid.substr(5, 4);
    };
  }).filter('runDuration', function() {
    return function(run) {
      var diff;
      if (run.completeTime) {
        diff = moment(run.completeTime).diff(run.startTime, 'hours');
      } else {
        diff = moment(run.touchTime).diff(run.startTime, 'hours');
      }
      return diff || '<1';
    };
  });

  app.controller('MarioRunCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.selrun = null;
    $scope.sampi = 0;
    $scope.samples = null;
    $http.get('/api/get-runs').success(function(data) {
      $scope.runs = data;
      return $scope.runTable = _.indexBy($scope.runs, 'fcid');
    });
    $scope.refreshRuns = function() {
      return $http.get('/api/get-runs').success(function(runs) {
        var run, _i, _len, _ref;
        for (_i = 0, _len = runs.length; _i < _len; _i++) {
          run = runs[_i];
          $scope.runTable[run.fcid].preprocess = run.preprocess;
          $scope.runTable[run.fcid].state = run.state;
        }
        return $http.post('/api/get-samples', {
          fcid: (_ref = $scope.selrun) != null ? _ref.fcid : void 0
        }).success(function(data) {
          return $scope.samples = data;
        });
      });
    };
    $scope.selectRun = function(run) {
      var r, _i, _len, _ref, _ref1, _ref2;
      $scope.samples = null;
      _ref = $scope.runs;
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        r = _ref[_i];
        r.selected = false;
      }
      $scope.selrun = run;
      $scope.selrun.selected = true;
      $http.post('/api/get-samples', {
        fcid: (_ref1 = $scope.selrun) != null ? _ref1.fcid : void 0
      }).success(function(data) {
        return $scope.samples = data;
      });
      return $http.post('/api/get-callsrc', {
        fcid: (_ref2 = $scope.selrun) != null ? _ref2.fcid : void 0
      }).success(function(data) {
        var _ref2;
        return (_ref2 = $scope.selrun) != null ? _ref2.callsrc = data : void 0;
      });
    };
    $scope.invokePreprocess = function() {
      return $http.post('/api/invoke-preprocess', {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          window.alert(data.toString());
        }
        return console.log(data);
      });
    };
    $scope.invokeAnalysis = function() {
      return $http.post('/api/invoke-analysis', {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          window.alert(data.toString());
        }
        return console.log(data);
      });
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshRuns();
      }), 5000);
    }
  });

}).call(this);
