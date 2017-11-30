package sharedcart

import (
	"bytes"
	"carts/models"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"net/http"
)

// CreateCart Create New Shared's Cart
func CreateCart(c *gin.Context) {

	jsonRequest, _ := ioutil.ReadAll(c.Request.Body)

	request := &models.CreateSharedCartRequest{}
	err := json.Unmarshal([]byte(jsonRequest), request)
	if err != nil {
		fmt.Println("Error Unmarshalling: ", err)
		c.String(http.StatusInternalServerError, "")
		return
	}

	reqAdminId := request.AdminId
	cartName := request.CartName
	c.Header("Content-Type", "application/json; charset=utf-8")

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}
	defer session.Close()

	// Create Empty Cart
	var products []models.Product

	cartId := bson.NewObjectId()
	var arrGroupUsers []string
	arrGroupUsers = append(arrGroupUsers, reqAdminId)

	emptySharedCart := models.SharedCart{
		Id:         cartId,
		AdminId:    reqAdminId,
		CartName:   cartName,
		GroupUsers: arrGroupUsers,
		Products:   products,
	}

	err = collection.Insert(emptySharedCart)

	sendCartCreatedEvent(c.Param("reqUserId"), emptySharedCart)

	resBody := models.CreateSharedCartResponse{
		CartId:     cartId.Hex(),
		Link:       models.LinkSharedCart + "/" + cartId.Hex(),
		InviteLink: models.LinkSharedCart + "/" + cartId.Hex() + "/join",
	}

	resJSON, _ := json.Marshal(resBody)
	c.String(http.StatusCreated, string(resJSON))
}

// GetCart get shared cart information
func GetCart(c *gin.Context) {

	c.Header("Content-Type", "application/json; charset=utf-8")

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}
	defer session.Close()

	var sharedCart models.SharedCart
	err = collection.FindId(bson.ObjectIdHex(c.Param("cartId"))).One(&sharedCart)

	if err != nil {
		c.String(http.StatusNotFound, "{\"Error\": \"Could no cart found for this id\"}")
		return
	}

	uj, _ := json.Marshal(sharedCart)

	c.String(http.StatusOK, string(uj))
}

// DeleteCart Delete shared cart
func DeleteCart(c *gin.Context) {
	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		return
	}
	defer session.Close()

	err = collection.Remove(bson.M{"_id": bson.ObjectIdHex(c.Param("cartId"))})

	if err != nil {
		c.String(http.StatusNotFound, "{\"Error\": \"Could not delete cart\"}")
		return
	}

	c.String(http.StatusOK, "")
}

// AddProduct to User cart
func AddProduct(c *gin.Context) {
	jsonRequest, _ := ioutil.ReadAll(c.Request.Body)

	product := &models.Product{}
	err := json.Unmarshal([]byte(jsonRequest), product)
	if err != nil {
		fmt.Println("Error Unmarshalling: ", err)
		c.String(http.StatusInternalServerError, "Error Unmarshalling json")
		return
	}

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}

	defer session.Close()

	query := bson.M{"_id": bson.ObjectIdHex(c.Param("cartId"))}
	change := bson.M{"$push": bson.M{"products": product}}
	_, err = collection.Upsert(query, change)
	if err != nil {
		fmt.Println("Error While inserting: ", err)
		c.String(http.StatusInternalServerError, "error while inserting")
		return
	}

	sendAddProductEvent(c.Param("reqUserId"), c.Param("cartId"), *product)

	c.String(http.StatusOK, "")
}

// UpdateProduct User Cart
func UpdateProduct(c *gin.Context) {
	jsonRequest, _ := ioutil.ReadAll(c.Request.Body)

	product := &models.Product{}
	err := json.Unmarshal([]byte(jsonRequest), product)
	if err != nil {
		fmt.Println("Error Unmarshalling: ", err)
		c.String(http.StatusInternalServerError, "error while unmarshalling json")
		return
	}

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}
	defer session.Close()

	fmt.Println("ProductId: ", c.Param("productId"))
	count, _ := collection.Find(bson.M{"_id": bson.ObjectIdHex(c.Param("cartId")), "products.id": c.Param("productId")}).Count()

	if count == 0 {
		c.String(http.StatusNotFound, "")
		return
	}

	query := bson.M{"_id": bson.ObjectIdHex(c.Param("cartId")), "products.id": product.Id}
	change := bson.M{"$set": bson.M{"products.$": product}}
	err = collection.Update(query, change)

	if err != nil {
		fmt.Println("Error While updating: ", err)
		c.String(http.StatusInternalServerError, "error while updating in mongo")
		return
	}

	sendUpdateProductEvent(c.Param("reqUserId"), c.Param("cartId"), c.Param("productId"), *product)

	c.String(http.StatusOK, "")
}

// RemoveProduct User Cart
func RemoveProduct(c *gin.Context) {

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}
	defer session.Close()

	fmt.Println("ProductId: ", c.Param("productId"))

	query := bson.M{"_id": bson.ObjectIdHex(c.Param("cartId"))}
	change := bson.M{"$pull": bson.M{"products": bson.M{"id": c.Param("productId")}}}

	err = collection.Update(query, change)
	if err != nil {
		fmt.Println("Error While removing: ", err)
		c.String(http.StatusInternalServerError, "Error while removing product from mongodb")
		return
	}

	sendRemoveProductEvent(c.Param("reqUserId"), c.Param("cartId"), c.Param("productId"))

	c.String(http.StatusOK, "")
}

// PlaceOrder place order for the items in the cart
func PlaceOrder(c *gin.Context) {

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}
	defer session.Close()

	// Get full cart information before placing the order
	var sharedCart models.SharedCart
	err = collection.FindId(bson.ObjectIdHex(c.Param("cartId"))).One(&sharedCart)

	if err != nil {
		c.String(http.StatusNotFound, "{\"Error\": \"Could no cart found for this id\"}")
		return
	}

	b, err := json.Marshal(sharedCart)
	fmt.Println(string(b))

	if len(sharedCart.Products) == 0 {
		c.String(http.StatusNotFound, "{\"Error\": \"cart is empty\"}")
		return
	}

	//jsonValue, _ := json.Marshal(sharedCart.Products)
	//resp, err := http.Post(authAuthenticatorUrl, "application/json", bytes.NewBuffer(jsonValue))

	// Remove cart products from the cart
	var products []models.Product
	query := bson.M{"_id": bson.ObjectIdHex(c.Param("cartId"))}
	change := bson.M{"$set": bson.M{"products": products}}

	err = collection.Update(query, change)
	if err != nil {
		fmt.Println("Error While removing: ", err)
		c.String(http.StatusInternalServerError, "Error while removing product from mongodb")
		return
	}

	//TODO: Send order placed event to user activity log
	sendPlaceOrderEvent(c.Param("reqUserId"), sharedCart)

	c.String(http.StatusOK, "")
}

// AddUser add users to the shared cart
func AddUser(c *gin.Context) {
	jsonRequest, _ := ioutil.ReadAll(c.Request.Body)

	var arrUserId []string

	if err := json.Unmarshal([]byte(jsonRequest), &arrUserId); err != nil {
		fmt.Println("Error Unmarshalling: ", err)
		c.String(http.StatusInternalServerError, "Error Unmarshalling json")
	}

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}
	defer session.Close()

	query := bson.M{"_id": bson.ObjectIdHex(c.Param("cartId"))}
	change := bson.M{"$push": bson.M{"groupUsers": bson.M{"$each": arrUserId}}}
	_, err = collection.Upsert(query, change)
	if err != nil {
		fmt.Println("Error While inserting: ", err)
		c.String(http.StatusInternalServerError, "error while inserting")
		return
	}

	sendAddUserEvent("Cart Admin", c.Param("cartId"), arrUserId[0])

	c.String(http.StatusOK, "")

}

// RemoveUser remove users from shared cart
func RemoveUser(c *gin.Context) {

	session, collection, err := getMongoConnection()
	if err != nil {
		c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return
	}
	defer session.Close()

	fmt.Println("UserId: ", c.Param("userId"))

	query := bson.M{"_id": bson.ObjectIdHex(c.Param("cartId"))}
	change := bson.M{"$pull": bson.M{"groupUsers": c.Param("userId")}}

	err = collection.Update(query, change)
	if err != nil {
		fmt.Println("Error While removing user: ", err)
		c.String(http.StatusInternalServerError, "Error while removing group user from mongodb")
		return
	}

	sendRemoveUserEvent(c.Param("reqUserId"), c.Param("cartId"), c.Param("userId"))

	c.String(http.StatusOK, "")
}

func GetUsersAllSharedCart(userId string) []models.SharedCart {
	var sharedCarts []models.SharedCart

	session, collection, err := getMongoConnection()
	if err != nil {
		//c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		fmt.Println("MongoDB Connection Failed")
		return sharedCarts
	}
	defer session.Close()

	var arrUserId []string
	arrUserId = append(arrUserId, userId)

	err = collection.Find(bson.M{"groupUsers": bson.M{"$in": arrUserId}}).All(&sharedCarts)

	if err != nil {
		//c.String(http.StatusNotFound, "{\"Error\": \"Could no cart found for this id\"}")
		fmt.Println("no cart found for this id")
		return sharedCarts
	}
	return sharedCarts
}

func getMongoConnection() (mgo.Session, mgo.Collection, error) {

	var collection *mgo.Collection

	session, err := mgo.Dial(models.MongodbServer)
	if err != nil {
		fmt.Println("mongodb connection failed", err)
		//c.String(http.StatusInternalServerError, "")
		return *session, *collection, err
	}
	//defer session.Close()
	session.SetMode(mgo.Monotonic, true)
	collection = session.DB(models.MongodbDatabase).C(models.MongodbCollectionSharedCarts)

	return *session, *collection, nil
}

// --- Activity Log Events --- //

func sendCartCreatedEvent(reqUserId string, sharedcart models.SharedCart) {

	var dic map[string]string
	dic = make(map[string]string)

	products, _ := json.Marshal(sharedcart.Products)
	groupUsers, _ := json.Marshal(sharedcart.GroupUsers)

	dic["userid"] = sharedcart.AdminId
	dic["cartid"] = sharedcart.Id.Hex()
	dic["cartname"] = sharedcart.CartName
	dic["typeofcart"] = "shared"
	dic["products"] = string(products)
	dic["groupusers"] = string(groupUsers)
	dic["activity"] = "Cart Created"

	requestData, _ := json.Marshal(dic)

	_, err := http.Post(models.ActivityLogServerURL, "application/json", bytes.NewBuffer(requestData))

	if err != nil {
		fmt.Println("Could not send event to activity log server", err)
	}
}

func sendAddProductEvent(reqUserId string, cartId string, product models.Product) {

	var dic map[string]string
	dic = make(map[string]string)

	productString, _ := json.Marshal(product)

	// dic["userid"] = reqUserId
	dic["cartid"] = cartId
	dic["cartname"] = getCartNameFromCartId(cartId)
	dic["typeofcart"] = "shared"
	dic["products"] = string(productString)
	dic["activity"] = "Product Added"

	requestData, _ := json.Marshal(dic)

	_, err := http.Post(models.ActivityLogServerURL, "application/json", bytes.NewBuffer(requestData))

	if err != nil {
		fmt.Println("Could not send event to activity log server", err)
	}
}

func sendUpdateProductEvent(reqUserId string, cartId string, productId string, product models.Product) {
	var dic map[string]string
	dic = make(map[string]string)

	productString, _ := json.Marshal(product)

	// dic["userid"] = reqUserId
	dic["cartid"] = cartId
	dic["cartname"] = getCartNameFromCartId(cartId)
	dic["typeofcart"] = "shared"
	dic["product"] = string(productString)
	dic["activity"] = "Cart Updated"

	requestData, _ := json.Marshal(dic)

	_, err := http.Post(models.ActivityLogServerURL, "application/json", bytes.NewBuffer(requestData))

	if err != nil {
		fmt.Println("Could not send event to activity log server", err)
	}
}

func sendRemoveProductEvent(reqUserId string, cartId string, productId string) {
	var dic map[string]string
	dic = make(map[string]string)

	// dic["userid"] = reqUserId
	dic["cartid"] = cartId
	dic["cartname"] = getCartNameFromCartId(cartId)
	dic["typeofcart"] = "shared"
	dic["productId"] = productId
	dic["activity"] = "Product Removed"

	requestData, _ := json.Marshal(dic)

	_, err := http.Post(models.ActivityLogServerURL, "application/json", bytes.NewBuffer(requestData))

	if err != nil {
		fmt.Println("Could not send event to activity log server", err)
	}
}

func sendAddUserEvent(reqUserId string, cartId string, addedUser string) {
	var dic map[string]string
	dic = make(map[string]string)

	// dic["userid"] = reqUserId
	dic["cartid"] = cartId
	dic["cartname"] = getCartNameFromCartId(cartId)
	dic["typeofcart"] = "shared"
	dic["addedUserId"] = addedUser
	dic["activity"] = "User Added"

	requestData, _ := json.Marshal(dic)

	_, err := http.Post(models.ActivityLogServerURL, "application/json", bytes.NewBuffer(requestData))

	if err != nil {
		fmt.Println("Could not send event to activity log server", err)
	}
}

func sendRemoveUserEvent(reqUserId string, cartId string, removedUserId string) {
	var dic map[string]string
	dic = make(map[string]string)

	// dic["userid"] = reqUserId
	dic["cartid"] = cartId
	dic["cartname"] = getCartNameFromCartId(cartId)
	dic["typeofcart"] = "shared"
	dic["removedUserId"] = removedUserId
	dic["activity"] = "User Removed"

	requestData, _ := json.Marshal(dic)

	_, err := http.Post(models.ActivityLogServerURL, "application/json", bytes.NewBuffer(requestData))

	if err != nil {
		fmt.Println("Could not send event to activity log server", err)
	}
}

func sendPlaceOrderEvent(reqUserId string, sharedcart models.SharedCart) {
	var dic map[string]string
	dic = make(map[string]string)

	products, _ := json.Marshal(sharedcart.Products)
	groupUsers, _ := json.Marshal(sharedcart.GroupUsers)

	dic["userid"] = reqUserId
	dic["cartid"] = sharedcart.Id.Hex()
	dic["cartname"] = sharedcart.CartName
	dic["typeofcart"] = "shared"
	dic["products"] = string(products)
	dic["groupusers"] = string(groupUsers)
	dic["activity"] = "Order Placed"

	requestData, _ := json.Marshal(dic)

	_, err := http.Post(models.ActivityLogServerURL, "application/json", bytes.NewBuffer(requestData))

	if err != nil {
		fmt.Println("Could not send event to activity log server", err)
	}
}

func getCartNameFromCartId(cartId string) string {

	session, collection, err := getMongoConnection()
	if err != nil {
		//c.String(http.StatusInternalServerError, "MongoDB Connection Failed")
		return ""
	}

	var sharedCart models.SharedCart
	collection.FindId(bson.ObjectIdHex(cartId)).One(&sharedCart)

	defer session.Close()

	return sharedCart.CartName
}
