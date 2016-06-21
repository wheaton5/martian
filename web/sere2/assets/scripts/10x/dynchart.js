// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

"use strict";

var global_chart;

function main() {
	console.log("HELLO WORLD"); 
	google.charts.load('current', {'packages':['corechart']});
	google.charts.setOnLoadCallback(function() {

		global_chart= new google.visualization.LineChart(document.getElementById('chart1'));
	});


}

function chart_update() {
	var x = document.getElementById("chartx");
	var y = document.getElementById("charty");
	var where = document.getElementById("chartwhere");

	console.log(x.value);
	console.log(x);
	console.log(y);
	console.log(where);

	var url = "/api/xyplot?where="+encodeURIComponent(where.value)+"&x=" + encodeURIComponent(x.value) + "&y=" +
		encodeURIComponent(y.value);

	console.log(url);
	$.getJSON(url, function(data) {
		console.log("GOTDATA!");
		console.log(data)
		var data = google.visualization.arrayToDataTable(data.ChartData);
		var options = {title:'12345'};
		global_chart.draw(data, options);

	})

}



main();

