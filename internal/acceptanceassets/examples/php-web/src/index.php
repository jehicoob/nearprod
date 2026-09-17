<?php
declare(strict_types=1);
header('Content-Type: application/json');
echo json_encode(['message' => 'nearprod-php-v1', 'runtime' => PHP_VERSION], JSON_THROW_ON_ERROR);
