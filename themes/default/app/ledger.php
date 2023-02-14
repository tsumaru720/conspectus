<?php

class Document extends Theme {

    protected $pageTitle = 'Ledger';
    private $db = null;
    private $entityManager = null;

    public function __construct(&$main, &$twig, $vars) {

        $vars['page_title'] = $this->pageTitle;

        $this->db = $main->getDB();
        $this->entityManager = $main->getEntityManager();

        if ($vars['left_menu'] != 'all') {
            $vars['modifier'] = "=";
        } else {
            $vars['item_id'] = 0;
            $vars['modifier'] = ">";
        }
        $data = array(':item_id' => $vars['item_id']);

        if ($vars['type'] == 'asset') {
            $logQuery = $this->db->query("SELECT
                                            asset_id,
                                            description,
                                            deposit_value,
                                            asset_value,
                                            (asset_value - deposit_value) AS gain,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        ORDER BY
                                            yearMonth DESC,
                                            description ASC", $data);
            $payQuery = $this->db->query("SELECT
                                            asset_id,
                                            description,
                                            amount,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            DATE_FORMAT(epoch, '%Y') AS year,
                                            DATE_FORMAT(epoch, '%e %M %Y') AS fullDate,
                                            DATE_FORMAT(epoch, '%D') AS shortDate,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            payments
                                        LEFT JOIN asset_list ON payments.asset_id = asset_list.id
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        ORDER BY
                                            yearMonth DESC,
                                            epoch DESC,
                                            description ASC", $data);
            if ($vars['item_id'] > 0) {
                $entity = $this->entityManager->getAsset($vars['item_id']);
                $this->pageTitle = "Asset Ledger - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
                $vars['asset_class'] = $entity->getClass();
                $vars['asset_class_id'] = $entity->getClassID();
                $vars['closed'] = $entity->isClosed();
            }
        } elseif ($vars['type'] == 'class') {
            $logQuery = $this->db->query("SELECT
                                            asset_id,
                                            asset_list.description,
                                            deposit_value,
                                            asset_value,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id = :item_id
                                        ORDER BY
                                            yearMonth DESC,
                                            description ASC", $data);
            $payQuery = $this->db->query("SELECT
                                            asset_id,
                                            asset_list.description,
                                            amount,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            DATE_FORMAT(epoch, '%Y') AS year,
                                            DATE_FORMAT(epoch, '%e %M %Y') AS fullDate,
                                            DATE_FORMAT(epoch, '%D') AS shortDate,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            payments
                                        LEFT JOIN asset_list ON payments.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id = :item_id
                                        ORDER BY
                                            yearMonth DESC,
                                            epoch DESC,
                                            description ASC", $data);
            if ($vars['item_id'] > 0) {
                $entity = $this->entityManager->getClass($vars['item_id']);
                $this->pageTitle = "Class Ledger - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
            }
        }

        while ($log = $this->db->fetch($logQuery)) {
            $log['deposit_value'] = $this->prettify($log['deposit_value']);
            $log['asset_value'] = $this->prettify($log['asset_value']);
            $vars['log'][] = $log;
        }

        $yearTotals = [];
        while ($payment = $this->db->fetch($payQuery)) {
            if (!array_key_exists($payment['year'], $yearTotals)) {
                $yearTotals[$payment['year']] = 0;
            }
            $yearTotals[$payment['year']] += $payment['amount'];
            $payment['amount'] = $this->prettify($payment['amount']);
            $vars['payment'][] = $payment;
            $vars['payDates'][$payment['yearMonth']][$payment['asset_id']] = true;
        }

        foreach ($yearTotals as $k => $v) {
            $yearTotals[$k] = $this->prettify($v);
        }
        $vars['totals'] = $yearTotals;

        $this->vars = $vars;
        $this->document = $twig->load('ledger.html');

    }

    private function prettify($value) {
        return ($value < 0 ? '-£'.number_format($value * -1, 2) : '£'.number_format($value, 2));
        //TODO some kind of configurable currency
    }
}