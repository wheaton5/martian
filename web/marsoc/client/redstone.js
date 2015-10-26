(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('RedstoneCtrl', function($scope, $http, $interval) {
    var CFG;
    $scope.CFG = CFG = null;
    $http.get('/api/redstone/config').success(function(data) {
      if (typeof data === "string") {
        window.alert(data);
        return;
      }
      $scope.CFG = CFG = data;
      CFG.sourcekeys = _.keys(CFG.sources);
      $scope.addsource = 'longranger';
      return $scope.redstone = {
        from: '',
        to: '',
        desc: '',
        dtl: CFG.defaults.dtl,
        dlmax: CFG.defaults.dlmax,
        cost_est: 0,
        size_est: 0,
        samples: []
      };
    });
    $scope.newSample = function(data) {
      var f, files, lastSample, name, source, _i, _j, _len, _len1, _ref, _ref1;
      files = {};
      source = CFG.sources[data.source];
      _ref = _.keys(data.fileinfo);
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        f = _ref[_i];
        files[f] = {
          include: data.fileinfo[f].include,
          path: data.fileinfo[f].path,
          size: data.fileinfo[f].size
        };
      }
      if (data.source !== "folder") {
        if ($scope.redstone.samples.length > 0) {
          lastSample = $scope.redstone.samples[$scope.redstone.samples.length - 1];
          _ref1 = _.keys(files);
          for (_j = 0, _len1 = _ref1.length; _j < _len1; _j++) {
            f = _ref1[_j];
            if (lastSample.files[f] != null) {
              files[f].include = lastSample.files[f].include;
            }
          }
        }
      }
      if (data.idtype === 'lena') {
        name = data.bag.description;
      } else if (data.idtype === 'path') {
        name = data.id.split("/").reverse()[0];
      }
      name = name.replace(/\s+/g, '_').replace(/[^\d\w]+/g, '');
      return $scope.redstone.samples.push({
        source: source,
        sourcename: data.source,
        container: data.container,
        id: data.id,
        idtype: data.idtype,
        versions: data.versions.reverse(),
        version: data.versions[0],
        name: name,
        files: files,
        sizetotal: 0,
        hsize: '',
        cost: ''
      });
    };
    $scope.validate = function() {
      var desc, download_cost, f, gb, reqsamps, request, s, samps, sfiles, storage_cost, totalcost, totalsize, _i, _j, _k, _len, _len1, _len2, _ref, _ref1;
      reqsamps = [];
      samps = $scope.redstone.samples;
      totalsize = 0.0;
      totalcost = 0.0;
      for (_i = 0, _len = samps.length; _i < _len; _i++) {
        s = samps[_i];
        s.sizetotal = 0;
        _ref = _.keys(s.files);
        for (_j = 0, _len1 = _ref.length; _j < _len1; _j++) {
          f = _ref[_j];
          if (s.files[f].include) {
            s.sizetotal += s.files[f].size;
          }
        }
        totalsize += s.sizetotal;
        s.hsize = Humanize.fileSize(s.sizetotal);
        gb = s.sizetotal / (1024 * 1024 * 1024);
        storage_cost = gb * CFG.prices.s3_storage_per_gbmo * ($scope.redstone.dtl / 30);
        download_cost = gb * CFG.prices.s3_download_per_gb * $scope.redstone.dlmax;
        totalcost += storage_cost + download_cost;
        s.cost = Humanize.formatNumber(storage_cost + download_cost, 2);
        sfiles = [];
        if (s.sourcename !== "folder") {
          _ref1 = s.source.order;
          for (_k = 0, _len2 = _ref1.length; _k < _len2; _k++) {
            f = _ref1[_k];
            if (s.files[f].include) {
              sfiles.push(f);
            }
          }
        }
        reqsamps.push([s.idtype, s.id, s.container, s.version, s.name, sfiles.join('|')].join(','));
      }
      $scope.redstone.totalsize = Humanize.fileSize(totalsize);
      $scope.redstone.totalcost = '$' + Humanize.formatNumber(totalcost, 2);
      desc = $scope.redstone.desc;
      desc = desc.replace(/\s+/g, '_');
      desc = desc.replace(/[^\d\w]+/g, '');
      request = {
        date: moment().format(),
        from: $scope.redstone.from,
        to: $scope.redstone.to,
        desc: desc,
        dtl: $scope.redstone.dtl,
        dlmax: $scope.redstone.dlmax,
        totalsize: $scope.redstone.totalsize,
        totalcost: $scope.redstone.totalcost,
        samples: reqsamps
      };
      return $scope.output = angular.toJson(request, 4);
    };
    $scope.addSample = function() {
      var idtype, params, source;
      source = CFG.sources[$scope.addsource];
      idtype = $scope.addid[0] === '/' ? 'path' : 'lena';
      params = {
        source: $scope.addsource,
        type: source.type,
        id: $scope.addid,
        idtype: idtype,
        pname: source.pname,
        paths: source.paths
      };
      $http.post('/api/redstone/validate', params).success(function(data) {
        if (typeof data === "string") {
          window.alert(data);
          return;
        }
        $scope.newSample(data);
        return $scope.validate();
      });
      if (idtype === 'lena') {
        return $scope.addid = '' + (parseInt($scope.addid) + 1);
      } else {
        return $scope.addid = '';
      }
    };
    return $scope.close = function(i) {
      $scope.redstone.samples.splice(i, 1);
      return $scope.validate();
    };
  });

  app.directive('integer', function() {
    return {
      require: 'ngModel',
      link: function(scope, elm, attrs, ctrl) {
        return ctrl.$parsers.unshift(function(viewValue) {
          if (/^\-?\d+$/.test(viewValue)) {
            ctrl.$setValidity('integer', true);
            return parseInt(viewValue, 10);
          } else {
            ctrl.$setValidity('integer', false);
            return void 0;
          }
        });
      }
    };
  });

}).call(this);
