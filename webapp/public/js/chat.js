var last_message_id = 0

function append(msg) {
    var text = msg["content"]
    var name = msg["user"]["display_name"] + "@" + msg["user"]["name"]
    var date = msg["date"]
    var icon = msg["user"]["avatar_icon"]
    var p = $('<div class="media message"></div>')
		var body = $('<div class="media-body">')
    $('<img class="avatar d-flex align-self-start mr-3" alt="no avatar">').attr('src', '/icons/'+icon).appendTo(p)
    $('<h5 class="mt-0"></h5>').append($('<a></a>').attr('href', '/profile/'+msg["user"]["name"]).text(name)).appendTo(body)
    $('<p class="content"></p>').text(text).appendTo(body)
    $('<p class="message-date"></p>').text(date).appendTo(body)
    body.appendTo(p)
    p.appendTo("#timeline")
    last_message_id = Math.max(last_message_id, msg['id'])
    var messages = $("div[class*='media message']")
    if (100 < messages.length) {
        messages.slice(0, -100).remove()
    }
}

function go_bottom() {
    $(window).scrollTop($(document).height());
}

function get_channel_id() {
    var ar = window.location.pathname.split("/")
    for (var i = 0; i < ar.length; i++) {
        if (ar[i] == "channel" && ar.length != i + 1) {
            return ar[i + 1]
        }
    }
    console.log(ar)
    return "1"
}

function fetch_unread(callback) {
    $.ajax({
        dataType: "json",
        async: true,
        type: "GET",
        url: "/fetch",
        success: callback
    })
}

function get_message(callback) {
    channel_id = get_channel_id()
    msg_id = last_message_id

    if (channel_id == null) {
        console.error("channel_id is null")
        return
    }

    if (isNaN(msg_id)) {
        console.error("last_message_id is NaN")
        return
    }

    $.ajax({
        dataType: "json",
        async: true,
        type: "GET",
        url: "/message",
        data: {
            last_message_id: last_message_id,
            channel_id: channel_id
        },
        success: function(messages) {
            callback(messages)
        }
    })
}

function post_message(msg) {
    channel_id = get_channel_id()
    if (channel_id == null) {
        console.error("channel_id is null")
        return
    }

    $.ajax({
        async: true,
        type: "POST",
        url: "/message",
        data: {
            channel_id: channel_id,
            message: msg
        },
    })
}

function on_send_button() {
    var textarea = $("#chatbox-textarea")
    var msg = textarea.val()
    if (msg == "") {
        return
    }
    post_message(msg)
    textarea.val("")
}

$(document).ready(function() {

    $("#chatbox-textarea").keydown(function(e) {
        // Enter was pressed without shift key
        if (e.keyCode == 13 && !e.shiftKey)
        {
            on_send_button()
            // prevent default behavior
            e.preventDefault()
        }
    });

    get_message(function(messages) {
        messages.forEach(append)

        var loading = false

        setInterval(function() {
            if (loading) return
            loading = true
            fetch_unread(function(json) {
                console.log(json)
                channel_id = get_channel_id()
                updated = false
                json.forEach(function(channel) {
                    current_channel = channel.channel_id == channel_id
                    if (current_channel && 0 < channel.unread) {
                      updated = true
                    }
                    var badge = $("#unread-" + channel.channel_id)
                    if (current_channel || channel.unread == 0) {
                      badge.text("")
                    } else {
                      badge.text(channel.unread.toString())
                    }
                })
                if (updated) {
                  get_message(function(new_messages) {
                      if (0 < new_messages.length) {
                          new_messages.forEach(append)
                          go_bottom()
                      }
                      loading = false
                  })
                } else {
                  loading = false
                }
            })
        }, 10)
    })
})
