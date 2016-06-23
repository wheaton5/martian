// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

"use strict";

var global_chart;
var global_table;

function main() {
	console.log("HELLO WORLD"); 
	google.charts.load('current', {'packages':['corechart', 'table']});
	google.charts.setOnLoadCallback(function() {

		if (document.getElementById('chart1')){
			global_chart= new google.visualization.LineChart(document.getElementById('chart1'));
		}
		if (document.getElementById('table1')){
			global_table = new google.visualization.Table(document.getElementById('table1'));
		}

	});


}

function compare_update() {
	var url = "/api/compare?base=" + document.getElementById("idold").value +
		"&new=" + document.getElementById("idnew").value +
		"&metrics_def=met1.json"
	
	$.getJSON(url, function(data) {
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {};
		global_table.draw(gdata, options)


	})


}
function table_update() {

	var url = "/api/plot?where=&columns=test_reports.id,SHA,userid,finishdate,sampleid,comments"

	$.getJSON(url, function(data) {
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {};
		global_table.draw(gdata, options)


	})


}
function chart_update() {
	var x = document.getElementById("chartx");
	var y = document.getElementById("charty");
	var where = document.getElementById("chartwhere");

	console.log(x.value);
	console.log(x);
	console.log(y);
	console.log(where);

	var url = "/api/plot?where="+encodeURIComponent(where.value)+"&columns=" + encodeURIComponent(x.value) + "," +
		encodeURIComponent(y.value);

	console.log(url);
	$.getJSON(url, function(data) {
		console.log("GOTDATA!");
		console.log(data)
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {title:data.Name};
		global_chart.draw(gdata, options);

	})

}



main();

