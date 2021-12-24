<?php

class Document extends Theme {

    protected $pageTitle = 'Analytics';
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
            $dataQuery = $this->db->query("SELECT
                                            SUM(deposit_value) AS deposit_total,
                                            SUM(asset_value) AS asset_total,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
                                            EXTRACT(YEAR FROM epoch) AS year
                                        FROM
                                            asset_log
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth,
                                            year
                                        ORDER BY
                                            yearMonth ASC", $data);
            $payQuery = $this->db->query("SELECT
                                            asset_id,
                                            description,
                                            amount,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            payments
                                        LEFT JOIN asset_list ON payments.asset_id = asset_list.id
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        ORDER BY
                                            yearMonth ASC,
                                            description ASC", $data);
            if ($vars['item_id'] > 0) {
                $entity = $this->entityManager->getAsset($vars['item_id']);
                $this->pageTitle = "Analytics - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
                $vars['asset_class'] = $entity->getClass();
                $vars['asset_class_id'] = $entity->getClassID();
                $vars['closed'] = $entity->isClosed();
            }
        } elseif ($vars['type'] == 'class') {
            $dataQuery = $this->db->query("SELECT
                                            SUM(deposit_value) AS deposit_total,
                                            SUM(asset_value) AS asset_total,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
                                            EXTRACT(YEAR FROM epoch) AS year
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth,
                                            year
                                        ORDER BY
                                            yearMonth ASC", $data);
            $payQuery = $this->db->query("SELECT
                                            asset_id,
                                            asset_list.description,
                                            amount,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            payments
                                        LEFT JOIN asset_list ON payments.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id = :item_id
                                        ORDER BY
                                            yearMonth ASC,
                                            description ASC", $data);
            if ($vars['item_id'] > 0) {
                $entity = $this->entityManager->getClass($vars['item_id']);
                $this->pageTitle = "Analytics - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
            }
        }

        $vars['payments'] = [];
        while ($payment = $this->db->fetch($payQuery)) {
            if (array_key_exists($payment['yearMonth'],$vars['payments'])) {
                $vars['payments'][$payment['yearMonth']] += $payment['amount'];
            } else {
                $vars['payments'][$payment['yearMonth']] = $payment['amount'];
            }
        }

        $payTally = 0;
        //TODO Tidy up this whole mess of code - a lot of it can be put in generic functions
        while ($period = $this->db->fetch($dataQuery)) {
            $period['value_delta'] = 0;
            $period['growth_factor'] = 0;

            if (array_key_exists($period['yearMonth'],$vars['payments'])) {
                $payTally += $vars['payments'][$period['yearMonth']];
            }
            $period['asset_total_adj'] = $period['asset_total'] + $payTally;

            if (!isset($vars['periodData'][$period['year']])) {
                if ($period['year'] == date("Y")) {
                    $vars['periodData'][$period['year']]['label'] = "YTD";
                } else {
                    $vars['periodData'][$period['year']]['label'] = $period['year'];
                }
                $vars['periodData'][$period['year']]['twr'] = 0;
                $vars['periodData'][$period['year']]['start'] = $period['asset_total'];
            }

            if (isset($last)) {
                $period['value_delta'] = number_format($period['deposit_total'] - $last['deposit_total'], 2, '.', '');

                if (($period['asset_total'] != 0) && ($last['asset_total'] != 0)) {
                    $period['growth_factor'] = $period['asset_total_adj'] / ($last['asset_total_adj'] + $period['value_delta']);
                }

                if ($vars['periodData'][$period['year']]['twr'] == 0) {
                    if ($period['year'] == $last['year']) {
                        $vars['periodData'][$period['year']]['twr'] = $period['growth_factor'];
                    } else {
                        // Add TWR factor to previous year
                        if ($vars['periodData'][$last['year']]['twr'] == 0) {
                            $vars['periodData'][$last['year']]['twr'] = $period['growth_factor'];
                        } else {
                            $vars['periodData'][$last['year']]['twr'] = $vars['periodData'][$last['year']]['twr'] * $period['growth_factor'];
                        }
                        // Recalculate value percentage increase for the year
                        $vars['periodData'][$last['year']]['end'] = $period['asset_total'];
                        $start = $vars['periodData'][$last['year']]['start'];
                        $end = $vars['periodData'][$last['year']]['end'];
                        $vars['periodData'][$last['year']]['increase'] = number_format((($end / $start) - 1) * 100, 2, '.', '');
                    }
                } else {
                    $vars['periodData'][$period['year']]['twr'] = $vars['periodData'][$period['year']]['twr'] * $period['growth_factor'];
                }

            }

            $vars['periodData'][$period['year']]['end'] = $period['asset_total'];

            $start = $vars['periodData'][$period['year']]['start'];
            $end = $vars['periodData'][$period['year']]['end'];

            if ($start != 0) {
                $vars['periodData'][$period['year']]['increase'] = number_format((($end / $start) - 1) * 100, 2, '.', '');
            } else {
                $vars['periodData'][$period['year']]['start'] = $period['asset_total'];
                $vars['periodData'][$period['year']]['increase'] = 0;
            }

            $last = $period;
        }

        if (array_key_exists('periodData', $vars)) {
            foreach ($vars['periodData'] as $key => $value) {
                if ($value['twr'] != 0) {
                    $value['twr'] = ($value['twr'] - 1) * 100;
                    $vars['periodData'][$key]['twr'] = number_format($value['twr'],2);
                }
            }
        }

        $this->vars = $vars;
        $this->document = $twig->load('analytics.html');
    }

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
