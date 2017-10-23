#!/usr/bin/env plackup

use 5.26.1;
use strict;
use warnings;
use utf8;

use FindBin;
use lib "$FindBin::Bin/lib";
use File::Basename;
use Plack::Builder;
use Isubata::Web;

my $root_dir = File::Basename::dirname(__FILE__);
my $app      = Isubata::Web->psgi($root_dir);

builder {
    enable 'Static',
        path => qr!^/(?:(?:css|js|fonts)/|favicon\.ico$)!,
        root => $root_dir . '/../public';

    enable 'Session::Cookie',
        session_key => "session",
        secret      => 'secretonymoris',
        httponly    => 1,
        expires     => 360000;

    $app;
};

