var scShoppingCartServerURL = "http://localhost/carts";
var scTempUserId = "anuj";

var scSelectedCartIndex = 0;
var scCartModel;

function scCartSelectionChanged(e){
    scSelectedCartIndex =  parseInt(e.target.id);
    var event = new Event('productsloaded');
    document.dispatchEvent(event);
}


function scSendRequest(strType, strUrl, productData, callback){
    $.ajax({
            type: strType,
            url: strUrl, 
            data: productData,
            error: function(xhr, status, error) {
                //console.log("Error While Adding Product to cart ", xhr.responseText);
                callback(true);
             },
            success: function(result) {
                console.log("Response Arrived !!!!!!!!!", result);
                callback(true, result);
            },
            dataType: "json"
        });
}


function scSendRequestUpdateItemUserCart(userId, product, callback){
    
    var url = scShoppingCartServerURL + "/user/" + userId + "/product/" + product.id;
    var data = {  
        "id"		: product.id,
        "quantity"  : product.quantity,
        "name"      : product.item_name,
        "price"     : product.amount
    };
    scSendRequest("PUT", url, JSON.stringify(data), callback);
}

function scSendRequestAddItemUserCart(userId, product, callback){
    
    var url = scShoppingCartServerURL + "/user/" + userId + "/product";
    var data = {  
        "id"		: product.id,
        "quantity"  : product.quantity,
        "name"      : product.item_name,
        "price"     : product.amount
    };
    scSendRequest("POST", url, JSON.stringify(data), callback);
}

    

function scSendRequestAddItemSharedCart(cartId, product, callback){
    
    var url = scShoppingCartServerURL + "/shared/" + cartId + "/product";
    var data = {  
        "id"		: product.id,
        "quantity"  : product.quantity,
        "name"      : product.item_name,
        "price"     : product.amount,
        "addedBy"   : scTempUserId
    };
    scSendRequest("POST", url, JSON.stringify(data), callback);
}


function scSendRequestUpdateItemSharedCart(cartId, product, callback){
    var url = scShoppingCartServerURL + "/shared/" + cartId + "/product/" + product.id;
    var data = {  
        "id"		: product.id,
        "quantity"  : product.quantity,
        "name"      : product.item_name,
        "price"     : product.amount,
        "addedBy"   : scTempUserId
    };
    scSendRequest("PUT", url, JSON.stringify(data), callback);
};

function scReloadCart(bSuccessful){
    if(bSuccessful){
        //self.fire('add', idx, product, isExisting);
        var event = new Event('cartReloadRequired');
        document.dispatchEvent(event);
    }
}

function scSendRequestPlaceOrderUserCart(userId, callback){
    var url = scShoppingCartServerURL + "/user/" + userId + "/order";
    scSendRequest("POST", url, JSON.stringify({}), callback);
}

function scSendRequestPlaceOrderSharedCart(cartId, callback){
    var url = scShoppingCartServerURL + "/shared/" + cartId + "/order";
    scSendRequest("POST", url, JSON.stringify({}), callback);
}

function scSendRequestGetCartDetails(cartId, callback){
    var url = scShoppingCartServerURL + "/shared/" + cartId;
    scSendRequest("GET", url, JSON.stringify({}), callback);
}

function  scSendRequestDeleteUserFromCart(cartId, userId, callback){
    var url = scShoppingCartServerURL + "/shared/" + cartId + "/user/" + userId;
    scSendRequest("DELETE", url, JSON.stringify({}), callback);
}