package reports

import (
	"fmt"
	"os"

	quickchartgo "github.com/henomis/quickchart-go"
)

func createChart() {
	chartConfig := `{
		type: 'bar',
		data: {
			labels: ["user1", "user2", "user3"],
			datasets: [{
			label: 'Users',
			data: [2, 3, 5, 1]
			}]
		}
	}`
	qc := quickchartgo.New()
	qc.Config = chartConfig
	qc.Width = 500
	qc.Height = 300
	qc.Version = "2.9.4"
	qc.BackgroundColor = "black"

	// Print the chart URL
	fmt.Println(qc.GetUrl())

	// Or write it to a file
	file, err := os.Create("output.png")
	if err != nil {
		panic(err)
	}

	defer file.Close()
	qc.Write(file)
}
