(function() {
  var actualSeconds, app, callApiWithConfirmation, predictedSeconds;

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
      d = 379 * (total - 12) + 21513;
    } else {
      d = 268 * total + 7080;
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

  callApiWithConfirmation = function($scope, $url) {
    var fcid;
    $scope.showbutton = false;
    fcid = window.prompt("Please type the flowcell ID to confirm");
    if (fcid === $scope.selrun.fcid) {
      return $http.post($url, {
        fcid: $scope.selrun.fcid
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          return window.alert(data.toString());
        }
      });
    } else {
      return window.alert("Incorrect flowcell ID");
    }
  };

  app.controller('MartianRunCtrl', function($scope, $http, $interval) {
    $scope.admin = admin;
    $scope.urlprefix = admin ? '/admin' : '';
    $scope.selrun = null;
    $scope.sampi = 0;
    $scope.samples = null;
    $scope.showbutton = true;
    $scope.autoinvoke = {
      button: true,
      state: false
    };
    $http.get('/api/get-runs').success(function(data) {
      $scope.runs = data;
      return $scope.runTable = _.indexBy($scope.runs, 'fcid');
    });
    if ($scope.admin) {
      $http.get('/api/get-auto-invoke-status').success(function(data) {
        return $scope.autoinvoke.state = data.state;
      });
    }
    $scope.refreshRuns = function() {
      $http.get('/api/get-runs').success(function(runs) {
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
      if ($scope.admin) {
        return $http.get('/api/get-auto-invoke-status').success(function(data) {
          $scope.autoinvoke.state = data.state;
          return $scope.autoinvoke.button = true;
        });
      }
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
    $scope.wipePreprocess = function() {
      return callApiWithConfirmation($scope, '/api/wipe-preprocess');
    };
    $scope.killPreprocess = function() {
      return callApiWithConfirmation($scope, '/api/kill-preprocess');
    };
    $scope.archivePreprocess = function() {
      $scope.showbutton = false;
      return $http.post('/api/archive-preprocess', {
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
    $scope.wipeSamples = function() {
      return callApiWithConfirmation($scope, '/api/wipe-fcid-samples');
    };
    $scope.killSamples = function() {
      return callApiWithConfirmation($scope, '/api/kill-fcid-samples');
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
    $scope.someFail = function() {
      return _.some($scope.samples, function(s) {
        return s.psstate === 'failed';
      });
    };
    $scope.someRunning = function() {
      return _.some($scope.samples, function(s) {
        return s.psstate === 'running';
      });
    };
    $scope.getAutoInvokeClass = function() {
      if ($scope.autoinvoke.state) {
        return "complete";
      } else {
        return "failed";
      }
    };
    $scope.setAutoInvoke = function() {
      $scope.autoinvoke.button = false;
      return $http.post('/api/set-auto-invoke-status', {
        state: !$scope.autoinvoke.state
      }).success(function(data) {
        $scope.refreshRuns();
        if (data) {
          return window.alert(data.toString());
        }
      });
    };
    if (admin) {
      return $interval((function() {
        return $scope.refreshRuns();
      }), 5000);
    }
  });

}).call(this);
