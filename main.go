package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"fmt"
	"io/ioutil"
	"encoding/json"
	"strings"

	"strconv"
	"time"

)


const _URL_ML_ARG string = "https://api.mercadolibre.com/sites/MLA/"
const _REQUEST_TYPE = "REQUEST_TYPE"
const MAXLIMITPAG = "200" // el maximo de items que me devuelve la api es 200
type urlInfoClient struct{ // un pequeño Wrapper con info de como armar la url para realizar el get a la API de mercado libre
	ID string
	action string
	params map[string]string



}


type Paginacion struct {
	Total  int64 `json:"total"`
	Offset int64 `json:"offset"`
	Limit  int64 `json:"limit"`
}


type DetalleRespFromApi struct {
	Price float64 `json:"price"`
	CantidadVendida int `json:"sold_quantity"`
	Condicion string `json:"condition"`


}


type JsonRespFromAPI struct {
	Paging Paginacion `json:"paging"`
	Result []DetalleRespFromApi  `json:"results"`

}

type JsonRespToClient struct {

	Max       string `json:"max"`
	Suggested string `json:"Suggested"`
	Min 	  string `json:"min"`

}



func  (jsResp *JsonRespFromAPI)  getPriceAVG() float64{
	var totalPrice float64
	var count int = 0
	for _,detailResp := range jsResp.Result{
		if detailResp.CantidadVendida > 0  { // que por lo menos haya vendido algo, nos aseguramos (aunque no del todo) que no sea publicacion basura
			totalPrice += detailResp.Price
			count++
		}
	}


	return totalPrice / float64(count)

}

func  genJsonPriceResp(values [2]int64) (rClient JsonRespToClient ){

	if values[0] > values[1]{
		rClient.Max = strconv.FormatInt(values[0],10)
		rClient.Min = strconv.FormatInt(values[1],10)

	}else{
		rClient.Max = strconv.FormatInt(values[1],10)
		rClient.Min = strconv.FormatInt(values[0],10)
	}


	rClient.Suggested = strconv.FormatInt((values[0] + values[1] ) / 2,10)

	return




}



type APIML struct {
	api_Method  map[string]interface{}
	urlInfoClient


}

func ( ml*  APIML) initAPI (){
	ml.api_Method = make(map[string]interface{})
	ml.api_Method["categories"] = ml.getInfoForCategory


}

func (ml* APIML) consumeAPIMethod(requestType string,ID string,action string,extraParam string, apiRespChan chan JsonRespFromAPI){

	apiMethod := ml.api_Method[requestType]
	// split params

	mapParams := make(map[string]string)

	for _, maper := range strings.Split(extraParam,"&") {
		slideParam :=  strings.Split(maper ,"=")
		mapParams[slideParam[0]] = slideParam[1]
	}


	ml.urlInfoClient =  urlInfoClient{ID, action, mapParams}

	apiChan := apiMethod.(func() (JsonRespFromAPI))()

	apiRespChan <- apiChan




}



func getURL(infoCli urlInfoClient) (string){
	var URLparameter string

	URLparameter += _URL_ML_ARG + infoCli.action + "?" + _REQUEST_TYPE + "=" + infoCli.ID

	for key,value :=  range infoCli.params{
			URLparameter += "&" + key + "=" + value
	}
	return URLparameter
}

func getJsonForResponse (resp* http.Response) (respondObj*JsonRespFromAPI){
	bResp, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(bResp, &respondObj)

	return

}

func (ml* APIML) getInfoForCategory() (JsonRespFromAPI ){
	url := strings.Replace(getURL(ml.urlInfoClient),_REQUEST_TYPE,"category",-1)
	var apiJsonResp* JsonRespFromAPI

	for { // LA API puede darme algun timeout
		resp, err := http.Get(url)
		fmt.Println("finish send "  + url)
		if err != nil {
			time.Sleep(500 * time.Millisecond) // espero un toque
			//return JsonRespFromAPI{}, err

		}else {
			apiJsonResp = getJsonForResponse(resp)
			break
		}

	}
	return  *apiJsonResp



}


/*********************************************************

  se hacen dos llamadas, primero tomamos los primeros 200 de precio mas bajo y despues los primeros 200 de precio mas alto (200 es el maximo que de item que me puede traer la api ), sacamos un promedio de ambos
   y mostramos precio menor,mayor y sugerido se hace de esta manera porque noto que ciertos usuarios cargan cualquier ganzada, ejemplo cargan mal la cateogria del producto,
   tambien esta la posibilidad de entrar en una categoria como  bebidas y encontrarte con un tipo que vende una tapa de botella a 50 centavos, o peor, vende una botella con el "aliento de Jesus" a 3800 pesos,
   dado esto tomar el precio mas bajo y el mas alto literalmente no sirve....

**********************************************************
*/

func main() {
	apiMl := APIML{}
	apiMl.initAPI()

	router := gin.Default()
	var count int64 = 0


	//resulTotal :=  strconv.FormatInt(respondObj.Paging.Total - 1, 10)


	router.GET("categories/:ID/prices/", func(c *gin.Context) {

		catID := c.Param("ID")
		//c.String(http.StatusOK, "Hello %s", catID)
		fmt.Println("llamda: "  + strconv.FormatInt(count,10))


		blockChan := make(chan interface{})
		//go func (){


		var minMax [2]int64
		apiResp := make(chan JsonRespFromAPI,1)
		timeout := make(chan bool, 1)
		go func(){
			time.Sleep(15 * time.Second)
			timeout <- true

		}()

		go func() {
			/** TODO: se puede hacer algo mejor y mas exacto en donde consumeAPIMethod llama a su metodo destino hasta consumir el tatal de item para la categoria,
				TODO:	o quizas el metodo llamante se podria llamar recursivamente

			*/
			apiMl.consumeAPIMethod("categories", catID, "search", "condition=new&sort=price_asc&limit="+MAXLIMITPAG, apiResp)
			apiMl.consumeAPIMethod("categories", catID, "search", "condition=new&sort=price_desc&limit="+MAXLIMITPAG, apiResp)
			close(apiResp)
		}()





		go func() {
			count++
			var i int = 0
			for {

				select {
				case resp := <-apiResp:

					fmt.Printf("%v ya procese el json que me dio la api de ML \n",count)
					priceAvg := resp.getPriceAVG()
					minMax[i] =  int64(priceAvg)
					//fmt.Println(priceAvg)
					i++
					if i == 2{ //
						fmt.Printf("llamada : %v - ya procese ambos request desbloqueo la salida \n",count)
						jsonPrice := genJsonPriceResp(minMax)
						blockChan <- jsonPrice
						i = 0
					}

				case <- timeout :
					blockChan  <- JsonRespToClient{"0","0","0"}
					fmt.Println("me case de esperar  ")

				default:
					time.Sleep(500 * time.Millisecond)


				}


			}
		}()






		/*	for resp := range apiResp{
					priceAvg := resp.getPriceAVG()
					minMax[i] = priceAvg
					i++
					fmt.Println(priceAvg)

				//c.String(http.StatusOK, "Hello %s", catID)
			}*/



		//}()

		respServer := <- blockChan
		fmt.Println(respServer)
		c.JSON(200, respServer)
		fmt.Println("mande la info")

	})

	router.Run(":8080")
}
