(function() {
  var app;

  app = angular.module('app', ['ui.bootstrap']);

  app.controller('RedstoneCtrl', function($scope, $http, $interval) {
    $scope.FILES = ['bam', 'vcf', 'svc', 'svn', 'svp', 'lou'];
    $scope.FILELABELS = {
      'bam': 'BAM',
      'vcf': 'VCF',
      'svc': 'SvC',
      'svn': 'SvN',
      'svp': 'SvP',
      'lou': 'Loupe'
    };
    $scope.samples = [];
    $scope.newSample = function() {
      return {
        lenaid: 0,
        version: 'HEAD',
        name: '',
        files: {}
      };
    };
    $scope.validate = function() {
      var f, out, outsamps, s, sfiles, _i, _j, _len, _len1, _ref, _ref1;
      outsamps = [];
      _ref = $scope.samples;
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        s = _ref[_i];
        sfiles = [];
        _ref1 = $scope.FILES;
        for (_j = 0, _len1 = _ref1.length; _j < _len1; _j++) {
          f = _ref1[_j];
          if (s.files[f]) {
            sfiles.push(f);
          }
        }
        outsamps.push([s.lenaid, s.version, s.name, sfiles.join('|')].join(','));
      }
      out = {
        from: $scope.from,
        to: $scope.to,
        desc: $scope.desc,
        dtl: $scope.dtl,
        samples: outsamps
      };
      return $scope.output = angular.toJson(out, 4);
    };
    return $scope.addSample = function() {
      var lastSample, newSample;
      newSample = $scope.newSample();
      if ($scope.samples.length > 0) {
        lastSample = $scope.samples[$scope.samples.length - 1];
        newSample.lenaid = lastSample.lenaid + 1;
        newSample.version = lastSample.version;
        newSample.files = _.cloneDeep(lastSample.files);
        newSample.name = lastSample.name;
      }
      return $scope.samples.push(newSample);
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
