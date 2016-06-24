// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

"use strict";

var global_chart;
var global_table;
var global_table_data;
var global_compare;
var global_metrics_db;
var global_metrics_table;


function main() {
	console.log("HELLO WORLD"); 
	google.charts.load('current', {'packages':['corechart', 'table']});
	google.charts.setOnLoadCallback(function() {

		global_chart= new google.visualization.LineChart(document.getElementById('plot1'));
		global_table = new google.visualization.Table(document.getElementById('table1'));
		global_compare = new google.visualization.Table(document.getElementById('compare1'));
		global_metrics_table= new google.visualization.Table(document.getElementById('list1'));
		google.visualization.events.addListener(global_metrics_table, 'select', metrics_list_click);

		pickwindow("table")
	});
	
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();
	$("#help").hide();

	$.getJSON("/api/list_metrics?metrics_def=met1.json", function(data) {
		global_metrics_db = data.ChartData;


	})

}

function pickwindow(w) {
	console.log(w)
	$("#table").hide();
	$("#compare").hide();
	$("#plot").hide();
	$("#help").hide();

	$("#" + w).show();

	if (w == "compare") {
		compare_update();
	}

	if (w == "table") {
		table_update();
	}

	if (w == "plot") {
		var mdata = google.visualization.arrayToDataTable(global_metrics_db);
		global_metrics_table.draw(mdata, {})

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
		var options = {allowHtml:true};
		colorize_table(data.ChartData,gdata)
		global_compare.draw(gdata, options)


	})


}


function table_update(mode) {
	var where = encodeURIComponent(document.getElementById("where").value);


	if (mode=="metrics") {
		var url = "/api/plotall?where=" + where + "&metrics_def=met1.json"
	} else {
		var url = "/api/plot?where=" + where + "&columns=test_reports.id,SHA,userid,finishdate,sampleid,comments"
	}

	$.getJSON(url, function(data) {
		global_table_data = data;
		console.log(data);
		var gdata = google.visualization.arrayToDataTable(data.ChartData);
		var options = {width: 1200};
		global_table.draw(gdata, options)
	})
}

function metrics_list_click() {
	var y = document.getElementById("charty");
	var idx = global_metrics_table.getSelection()[0].row;
	y.value = global_metrics_db[idx + 1];
}

function chart_update() {
	var x = document.getElementById("chartx");
	var y = document.getElementById("charty");
	var where = document.getElementById("where");

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
		if (labels[i] == columnname) {
			index = i;
			break;
		}
	}

	return data.ChartData[rownumber+1][index];
}

function colorize_table(data, datatable) {
	var diff_index;
	var labels = data[0];
	for (var i = 0; i < labels.length; i++) {
		if (labels[i] == 'Diff') {
			diff_index= i;
			break;
		}
	}

	for (var i = 1; i < data.length; i++) {
		var di = i - 1;
		
		if (data[i][diff_index] === false) {
			for (var j = 0; j < labels.length; j++) {
				datatable.setProperty(di, j, 'style', 'color:red;')
			}
		}
	}
}

main();
