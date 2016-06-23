// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

"use strict";

var global_chart;
var global_table;
var global_table_data;
var global_compare;

function main() {
	console.log("HELLO WORLD"); 
	google.charts.load('current', {'packages':['corechart', 'table']});
	google.charts.setOnLoadCallback(function() {

		global_chart= new google.visualization.LineChart(document.getElementById('plot1'));
		global_table = new google.visualization.Table(document.getElementById('table1'));
		global_compare = new google.visualization.Table(document.getElementById('compare1'));

	});
	
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();



}

function pickwindow(w) {
	console.log(w)
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();

	$("#" + w).show();

	if (w == "compare") {
		compare_update();
	}

	if (w == "table") {
		table_update();
	}

	if (w == "plot") {

	}
}

function compare_update() {

	var selected = global_table.getSelection();
	console.log(selected)
	var idold = get_data_at_row(global_table_data, "test_reports.id", selected[0].row);
	var idnew= get_data_at_row(global_table_data, "test_reports.id", selected[1].row);

	
	/*var url = "/api/compare?base=" + document.getElementById("idold").value +
		"&new=" + document.getElementById("idnew").value +
		"&metrics_def=met1.json"
		*/
	var url = "/api/compare?base=" + idold + 
		"&new=" + idnew + 
		"&metrics_def=met1.json"
	
	console.log(url)
	$.getJSON(url, function(data) {
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {};
		global_compare.draw(gdata, options)


	})


}
function table_update() {

	var url = "/api/plot?where=&columns=test_reports.id,SHA,userid,finishdate,sampleid,comments"

	$.getJSON(url, function(data) {
		global_table_data = data;
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

function get_data_at_row(data, columnname, rownumber) {
	var labels = data.ChartData[0];

	var index;
	for (var i = 0; i < labels.length; i++) {
		if (labels[i] = columnname) {
			index = i;
			break;
		}
	}

	return data.ChartData[rownumber+1][index];
}

main();
