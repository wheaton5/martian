(function() {
  var actualSeconds, app, predictedSeconds;

  actualSeconds = function(run) {
    var d;
    if (run.completeTime) {
      d = moment(run.completeTime).diff(run.startTime);
    } else {
      d = moment(run.touchTime).diff(run.startTime);
    }
    return moment.duration(d / 1000, 'seconds');
  };

  predictedSeconds = function(run) {
    var d, reads, total;
    reads = run.runinfoxml.Run.Reads.Reads;
    total = _.reduce(reads, function(sum, read) {
      return sum + read.NumCycles;
    }, 0);
    if (run.seqcerName.indexOf("hiseq") === 0) {
      d = 314 * total + 30960;
    } else {
      d = 249 * total + 6060;
    }
    return moment.duration(d, 'seconds');
  };

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
  }).filter('cycleInfo', function() {
    return function(selrun) {
      var readLens, reads, total;
      reads = selrun.runinfoxml.Run.Reads.Reads;
      readLens = _.map(reads, function(read) {
        return read.NumCycles;
      }).join(", ");
      total = _.reduce(reads, function(sum, read) {
        return sum + read.NumCycles;
      }, 0);
      return "" + readLens + " (" + total + ")";
    };
  }).filter('runDuration', function() {
    return function(run) {
      var dact, dpred, pctg;
      dact = actualSeconds(run);
      if (dact == null) {
        return '<1';
      }
      dpred = predictedSeconds(run);
      pctg = Math.floor(dact / dpred * 100.0);
      return "" + (dact.hours() + 24 * dact.days()) + "h " + (dact.minutes()) + "m (" + pctg + "%)";
    };
  }).filter('runPrediction', function() {
    return function(run) {
      var dact, dpred, eta;
      dact = actualSeconds(run);
      dpred = predictedSeconds(run);
      eta = moment(run.startTime).add(dpred).format("ddd MMM D, h:mm a");
      return "" + (dpred.hours() + 24 * dpred.days()) + "h " + (dpred.minutes()) + "m (" + eta + ")";
    };
  });

  app.controller('MarioRunCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.selrun = null;
    $scope.sampi = 0;
    $scope.samples = null;
    $scope.showbutton = true;
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
          $scope.samples = data;
          return $scope.showbutton = true;
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
      $scope.showbutton = false;
      return $http.post('/api/invoke-preprocess', {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    $scope.invokeAnalysis = function() {
      $scope.showbutton = false;
      return $http.post('/api/invoke-analysis', {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    $scope.archiveSamples = function() {
      $scope.showbutton = false;
      return $http.post('/api/archive-fcid-samples', {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    $scope.unfailSamples = function() {
      $scope.showbutton = false;
      return $http.post('/api/restart-fcid-samples', {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    $scope.allDone = function() {
      return _.every($scope.samples, function(s) {
        return s.psstate === 'complete';
      });
    };
    $scope.allFail = function() {
      return _.every($scope.samples, function(s) {
        return s.psstate === 'failed';
      });
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshRuns();
      }), 5000);
    }
  });

}).call(this);
