(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('RedstoneCtrl', function($scope, $http, $interval) {
    var FILELABELS, FILEPATHS, FILES, S3_DOWNLOAD_PRICE_PER_GB, S3_STORAGE_PRICE_PER_GB;
    $scope.FILES = FILES = ['bam', 'vcu', 'vcg', 'svc', 'svn', 'svp', 'lou'];
    $scope.FILELABELS = FILELABELS = {
      'bam': 'BAM',
      'vcu': 'VCF universal',
      'vcg': 'VCF upgrade',
      'svc': 'SV calls',
      'svn': 'SV candidates',
      'svp': 'SV phasing',
      'lou': 'Loupe'
    };
    FILEPATHS = {
      'bam': 'PHASER_SVCALLER_PD/PHASER_SVCALLER/ATTACH_PHASING/fork0/files/phased_possorted_bam.bam',
      'vcu': 'PHASER_SVCALLER_PD/_SNPINDEL_PHASER/ANALYZE_SNPINDEL_CALLS/fork0/files/varcalls.vcf.gz',
      'vcg': 'PHASER_SVCALLER_PD/PHASER_SVCALLER/_SNPINDEL_PHASER/ANALYZE_SNPINDEL_CALLS/fork0/files/varcalls.vcf.gz',
      'svc': 'PHASER_SVCALLER_PD/PHASER_SVCALLER/_STRUCTVAR_CALLER/ANALYZE_SV_CALLS/fork0/files/sv_calls.bedpe',
      'svn': 'PHASER_SVCALLER_PD/PHASER_SVCALLER/_STRUCTVAR_CALLER/ANALYZE_SV_CALLS/fork0/files/sv_candidates.bedpe',
      'svp': 'PHASER_SVCALLER_PD/PHASER_SVCALLER/PHASE_STRUCTVARS/fork0/files/sv_phasing.tsv',
      'lou': 'PHASER_SVCALLER_PD/PHASER_SVCALLER/LOUPE_PREPROCESS/fork0/files/output_for_loupe.loupe'
    };
    S3_STORAGE_PRICE_PER_GB = 0.03;
    S3_DOWNLOAD_PRICE_PER_GB = 0.09;
    $scope.redstone = {
      from: '',
      to: '',
      desc: '',
      dtl: 14,
      audience: 10,
      samples: []
    };
    $scope.newSample = function(data) {
      var f, files, lastSample, _i, _j, _len, _len1;
      files = {};
      for (_i = 0, _len = FILES.length; _i < _len; _i++) {
        f = FILES[_i];
        files[f] = {
          include: false,
          path: data.fileinfo[f].path,
          size: data.fileinfo[f].size
        };
      }
      if ($scope.redstone.samples.length > 0) {
        lastSample = $scope.redstone.samples[$scope.redstone.samples.length - 1];
        for (_j = 0, _len1 = FILES.length; _j < _len1; _j++) {
          f = FILES[_j];
          files[f].include = lastSample.files[f].include;
        }
      }
      return {
        lenaid: data.bag.id,
        versions: data.versions.reverse(),
        version: 'HEAD',
        name: data.bag.description,
        files: files,
        sizetotal: 0,
        hsize: '',
        cost: ''
      };
    };
    $scope.validate = function() {
      var download_cost, f, gb, reqsamps, request, s, samps, sfiles, storage_cost, totalcost, totalsize, _i, _j, _k, _len, _len1, _len2;
      reqsamps = [];
      samps = $scope.redstone.samples;
      totalsize = 0.0;
      totalcost = 0.0;
      for (_i = 0, _len = samps.length; _i < _len; _i++) {
        s = samps[_i];
        s.sizetotal = 0;
        for (_j = 0, _len1 = FILES.length; _j < _len1; _j++) {
          f = FILES[_j];
          if (s.files[f].include) {
            s.sizetotal += s.files[f].size;
          }
        }
        totalsize += s.sizetotal;
        s.hsize = Humanize.fileSize(s.sizetotal);
        gb = s.sizetotal / (1024 * 1024 * 1024);
        storage_cost = gb * S3_STORAGE_PRICE_PER_GB * ($scope.redstone.dtl / 30);
        download_cost = gb * S3_DOWNLOAD_PRICE_PER_GB * $scope.redstone.audience;
        totalcost += storage_cost + download_cost;
        s.cost = Humanize.formatNumber(storage_cost + download_cost, 2);
        sfiles = [];
        for (_k = 0, _len2 = FILES.length; _k < _len2; _k++) {
          f = FILES[_k];
          if (s.files[f].include) {
            sfiles.push(f);
          }
        }
        reqsamps.push([s.lenaid, s.version, s.name, sfiles.join('|')].join(','));
      }
      $scope.totalcost = Humanize.formatNumber(totalcost, 2);
      $scope.totalsize = Humanize.fileSize(totalsize);
      request = {
        from: $scope.redstone.from,
        to: $scope.redstone.to,
        desc: $scope.redstone.desc,
        dtl: $scope.redstone.dtl,
        audience: $scope.redstone.audience,
        samples: reqsamps
      };
      return $scope.output = angular.toJson(request, 4);
    };
    return $scope.addSample = function() {
      var params;
      params = {
        sid: $scope.newid,
        fpaths: FILEPATHS
      };
      $http.post('/api/redstone', params).success(function(data) {
        var newSample;
        if (typeof data === "string") {
          window.alert(data);
          return;
        }
        console.log(data);
        newSample = $scope.newSample(data);
        newSample.name = newSample.name.replace(/\s+/g, '_');
        newSample.name = newSample.name.replace(/[^\d\w]+/g, '');
        $scope.redstone.samples.push(newSample);
        return $scope.validate();
      });
      return $scope.newid = '' + (parseInt($scope.newid) + 1);
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
