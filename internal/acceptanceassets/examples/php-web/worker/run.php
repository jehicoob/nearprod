<?php
declare(strict_types=1);
// Loaded only once. Editing handler.php requires restarting this long-lived worker.
require __DIR__ . '/handler.php';
while (true) {
    echo date(DATE_ATOM) . ' ' . workMessage() . PHP_EOL;
    sleep(5);
}
