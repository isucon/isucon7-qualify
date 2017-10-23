package Isubata::Web;

use 5.26.1;
use strict;
use warnings;
use utf8;

use Kossy;
use DBIx::Sunny;
use Digest::SHA1 qw(sha1_hex);
use DateTime::Format::MySQL;
use List::Util qw(max);
use POSIX qw(ceil);
use Scalar::Util qw(looks_like_number);

use constant {
    AVATAR_MAX_SIZE => 1 * 1024 * 1024,
};

sub dbh {
    my $self = shift;

    $self->{_dbh} //= do {
        my %db = (
            host     => $ENV{ISUBATA_DB_HOST}      || 'localhost',
            port     => $ENV{ISUBATA_DB_PORT}      || 3306,
            username => $ENV{ISUBATA_DB_USER}      || 'root',
            password => $ENV{ISUBATA_DB_PASSWORD},
        );
        DBIx::Sunny->connect(
            "dbi:mysql:dbname=isubata;host=$db{host};port=$db{port}", $db{username}, $db{password}, {
                Callbacks => {
                    connected => sub {
                        my $dbh = shift;
                        $dbh->do(q{SET SESSION sql_mode='TRADITIONAL,NO_AUTO_VALUE_ON_ZERO,ONLY_FULL_GROUP_BY'});
                        $dbh->do(q{SET NAMES utf8mb4});
                        return;
                    },
                },
            },
        );
    };
}

sub get_user {
    my ($self, $user_id) = @_;

    return $self->dbh->select_row(qq{SELECT * FROM user WHERE id = ?}, $user_id);
}

sub add_message {
    my ($self, $channel_id, $user_id, $message) = @_;

    $self->dbh->query(
        qq{INSERT INTO message (channel_id, user_id, content, created_at) VALUES (?, ?, ?, NOW())},
        $channel_id, $user_id, $message,
    );
}

sub get_channel_list_info {
    my $self       = shift;
    my $channel_id = shift // 0;

    my $channels    = $self->dbh->select_all(qq{SELECT * FROM channel ORDER BY id});
    my $description = "";

    for my $channel (@$channels) {
        if ($channel->{id} == $channel_id) {
            $description = $channel->{description};
            last;
        }
    }

    return ($channels, $description);
}

my @LETTER_AND_DIGITS = split //, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";

sub random_string {
    my $n   = shift;
    my $res = "";
    for (1..$n) {
        $res .= $LETTER_AND_DIGITS[int(rand(scalar @LETTER_AND_DIGITS))];
    }
    return $res;
}

sub register {
    my ($self, $c, $name, $password) = @_;

    my $salt   = random_string(20);
    my $digest = sha1_hex($salt . $password);

    eval {
        $self->dbh->query(
            qq{INSERT INTO user (name, salt, password, display_name, avatar_icon, created_at) VALUES (?, ?, ?, ?, ?, NOW())},
            $name, $salt, $digest, $name, "default.png",
        );
    };
    if (my $e = $@) {
        if ($e =~ /DBD::mysql::st execute failed: Duplicate entry/) {
            $c->halt(409);
        }
        die $e; # rethrow
    }

    return $self->dbh->last_insert_id;
}

filter 'login_required' => sub {
    my $app = shift;
    sub {
        my ($self, $c) = @_;

        my $user_id = $c->req->session->{user_id};

        if (!$user_id) {
            return $c->redirect("/login", 303);
        }

        $c->stash->{user_id} = $user_id;

        my $user = $self->get_user($user_id);

        if (!$user) {
            return $c->redirect("/login", 303);
        }

        $c->stash->{user} = $user;

        $app->($self, $c);
    }
};

get '/initialize' => sub {
    my ($self, $c) = @_;

    $self->dbh->query("DELETE FROM user WHERE id > 1000");
    $self->dbh->query("DELETE FROM image WHERE id > 1001");
    $self->dbh->query("DELETE FROM channel WHERE id > 10");
    $self->dbh->query("DELETE FROM message WHERE id > 10000");
    $self->dbh->query("DELETE FROM haveread");

    $c->res->status(204);
    $c->res->body("");
};

get '/' => sub {
    my ($self, $c) = @_;
    if (exists $c->req->session->{user_id}) {
        return $c->redirect("/channel/1", 303);
    }
    $c->render('index.tx', {});
};

get '/channel/{channel_id:[0-9]+}' => [qw/login_required/] => sub {
    my ($self, $c) = @_;

    my $channel_id = $c->args->{channel_id};

    my ($channels, $description) = $self->get_channel_list_info($channel_id);

    $c->render('channel.tx', {
        channels    => $channels,
        description => $description,
        channel_id  => $channel_id
    });
};

get '/register' => sub {
    my ($self, $c) = @_;
    $c->render('register.tx', {});
};

post '/register' => sub {
    my ($self, $c) = @_;

    my $name     = $c->req->parameters->{name};
    my $password = $c->req->parameters->{password};

    if (!$name || !$password) {
        $c->halt(400);
    }

    my $user_id = $self->register($c, $name, $password);

    $c->req->session->{user_id} = $user_id;

    $c->redirect("/", 303);
};

get '/login' => sub {
    my ($self, $c) = @_;
    $c->render('login.tx', {});
};

post '/login' => sub {
    my ($self, $c) = @_;

    my $name     = $c->req->parameters->{name};
    my $password = $c->req->parameters->{password};

    my $row = $self->dbh->select_row(qq{SELECT * FROM user WHERE name = ?}, $name);
    if (!$row || $row->{password} ne sha1_hex($row->{salt} . $password)) {
        $c->halt(403);
    }

    $c->req->session->{user_id} = $row->{id};

    $c->redirect("/", 303);
};

get '/logout' => sub {
    my ($self, $c) = @_;
    $c->req->session_options->{expire}++;
    $c->redirect("/", 303);
};

post '/message' => sub {
    my ($self, $c) = @_;

    my $user_id    = $c->req->session->{user_id};
    my $message    = $c->req->parameters->{message};
    my $channel_id = $c->req->parameters->{channel_id};

    if (!$user_id || !$message || !$channel_id) {
        $c->halt(403);
    }

    $self->add_message($channel_id, $user_id, $message);

    $c->res->status(204);
    $c->res->body("");
};

get '/message' => sub {
    my ($self, $c) = @_;

    my $user_id = $c->req->session->{user_id};
    if (!$user_id) {
        $c->halt(403);
    }

    my $channel_id      = $c->req->parameters->{channel_id};
    my $last_message_id = $c->req->parameters->{last_message_id};

    my $rows = $self->dbh->select_all(
        qq{SELECT * FROM message WHERE id > ? AND channel_id = ? ORDER BY id DESC LIMIT 100},
        $last_message_id, $channel_id,
    );

    my @res;
    for my $row (@$rows) {
        my $user = $self->dbh->select_row(qq{SELECT name, display_name, avatar_icon FROM user WHERE id = ?}, $row->{user_id});
        unshift @res, {
            id      => $row->{id},
            user    => $user,
            date    => DateTime::Format::MySQL->parse_datetime($row->{created_at})->strftime("%Y/%m/%d %H:%M:%S"),
            content => $row->{content},
        };
    }

    if (0 < scalar @$rows) {
        my $max_message_id = max(map { $_->{id} } @$rows);

        $self->dbh->query(qq{
            INSERT INTO haveread (user_id, channel_id, message_id, updated_at, created_at)
            VALUES (?, ?, ?, NOW(), NOW())
            ON DUPLICATE KEY UPDATE message_id = ?, updated_at = NOW()
        }, $user_id, $channel_id, $max_message_id, $max_message_id);
    }

    $c->render_json(\@res);
};

get '/fetch' => sub {
    my ($self, $c) = @_;

    my $user_id = $c->req->session->{user_id};
    if (!$user_id) {
        $c->halt(403);
    }

    sleep(1);

    my @channel_ids = map { $_->{id} } @{$self->dbh->select_all(qq{SELECT id FROM channel})};

    my @res;
    for my $channel_id (@channel_ids) {
        my $row = $self->dbh->select_row(qq{SELECT * FROM haveread WHERE user_id = ? AND channel_id = ?}, $user_id, $channel_id);
        my $cnt = 0;
        if ($row) {
            $cnt = $self->dbh->select_one(qq{SELECT COUNT(*) as cnt FROM message WHERE channel_id = ? AND ? < id}, $channel_id, $row->{message_id});
        } else {
            $cnt = $self->dbh->select_one(qq{SELECT COUNT(*) as cnt FROM message WHERE channel_id = ?}, $channel_id);
        }
        push @res, {
            channel_id => $channel_id,
            unread     => $cnt,
        };
    }

    $c->render_json(\@res);
};

get '/history/{channel_id:[0-9]+}' => [qw/login_required/] => sub {
    my ($self, $c) = @_;

    my $channel_id = $c->args->{channel_id};
    my $page       = $c->req->parameters->{page} || 1;

    if (!looks_like_number($page)) {
        $c->halt(400);
    }

    my $cnt      = $self->dbh->select_one(qq{SELECT COUNT(*) as cnt FROM message WHERE channel_id = ?}, $channel_id);
    my $n        = 20;
    my $max_page = ceil($cnt / $n) || 1;

    if ($page < 1 || $max_page < $page) {
        $c->halt(400);
    }

    my $rows = $self->dbh->select_all(
        qq{SELECT * FROM message WHERE channel_id = ? ORDER BY id DESC LIMIT ? OFFSET ?},
        $channel_id, $n, ($page - 1) * $n,
    );

    my @messages;
    for my $row (@$rows) {
        my $user = $self->dbh->select_row(qq{SELECT name, display_name, avatar_icon FROM user WHERE id = ?}, $row->{user_id});

        unshift @messages, {
            id      => $row->{id},
            user    => $user,
            date    => DateTime::Format::MySQL->parse_datetime($row->{created_at})->strftime("%Y/%m/%d %H:%M:%S"),
            content => $row->{content},
        };
    }

    my ($channels) = $self->get_channel_list_info($channel_id);

    $c->render("history.tx", {
        channels   => $channels,
        channel_id => $channel_id,
        messages   => \@messages,
        max_page   => $max_page,
        page       => $page,
    });
};

get '/profile/:user_name' => [qw/login_required/] => sub {
    my ($self, $c) = @_;

    my ($channels) = $self->get_channel_list_info;

    my $user = $self->dbh->select_row(qq{SELECT * FROM user WHERE name = ?}, $c->args->{user_name});

    if (!$user) {
        $c->halt(404);
    }

    my $self_profile = ($c->stash->{user}{id} == $user->{id}) ? 1 : 0;

    $c->render("profile.tx", {
        channels     => $channels,
        user         => $user,
        self_profile => $self_profile,
    });
};

get '/add_channel' => [qw/login_required/] => sub {
    my ($self, $c) = @_;
    my ($channels) = $self->get_channel_list_info;
    $c->render('add_channel.tx', { channels => $channels });
};

post '/add_channel' => [qw/login_required/] => sub {
    my ($self, $c) = @_;

    my $name        = $c->req->parameters->{name};
    my $description = $c->req->parameters->{description};

    if (!$name || !$description) {
        $c->halt(400);
    }

    $self->dbh->query(
        qq{INSERT INTO channel (name, description, updated_at, created_at) VALUES (?, ?, NOW(), NOW())},
        $name, $description,
    );

    my $channel_id = $self->dbh->last_insert_id;

    $c->redirect("/channel/" . $channel_id, 303);
};

post '/profile' => [qw/login_required/] => sub {
    my ($self, $c) = @_;

    my $user_id = $c->req->session->{user_id};
    if (!$user_id) {
        $c->halt(403);
    }

    my $user = $self->get_user($user_id);
    if (!$user) {
        $c->halt(403);
    }

    my $display_name = $c->req->parameters->{display_name};
    my $avatar_name;
    my $avatar_data;

    my $file = $c->req->uploads->{avatar_icon};

    if ($file) {
        my $idx = index($file->basename, ".");
        my $ext = (0 <= $idx) ? substr($file->basename, $idx) : "";

        if (! grep { $ext eq $_ } qw/.jpg .jpeg .png .gif/) {
            $c->halt(400);
        }

        if (AVATAR_MAX_SIZE < $file->size) {
            $c->halt(400);
        }

        open my $fh, '<', $file->path;
        my $data = do { local $/; <$fh> }; # readall
        close($fh);

        my $digest = sha1_hex($data);

        $avatar_name = $digest . $ext;
        $avatar_data = $data;
    }

    if ($avatar_name && $avatar_data) {
        $self->dbh->query(qq{INSERT INTO image (name, data) VALUES (?, _binary ?)}, $avatar_name, $avatar_data);
        $self->dbh->query(qq{UPDATE user SET avatar_icon = ? WHERE id = ?}, $avatar_name, $user_id);
    }

    if ($display_name) {
        $self->dbh->query(qq{UPDATE user SET display_name = ? WHERE id = ?}, $display_name, $user_id);
    }

    $c->redirect("/", 303);
};

sub ext2mime {
    my $ext = shift;
    if (grep { $ext eq $_ } qw/.jpg .jpeg/) {
        return "image/jpeg"
    } elsif ($ext eq ".png") {
        return "image/png"
    } elsif ($ext eq ".gif") {
        return "image/gif"
    }
    return ''
}

get '/icons/:file_name' => sub {
    my ($self, $c) = @_;

    my $file_name = $c->args->{file_name};

    my $row  = $self->dbh->select_row(qq{SELECT * FROM image WHERE name = ?}, $file_name);
    my $idx  = index($file_name, ".");
    my $ext  = (0 <= $idx) ? substr($file_name, $idx) : "";
    my $mime = ext2mime($ext);

    if ($row && $mime) {
        $c->res->status(200);
        $c->res->content_type($mime);
        $c->res->body($row->{data});
        return $c->res;
    }

    $c->halt(404);
};

1;
