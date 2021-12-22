<?php

class Document extends Theme {

    protected $pageTitle = 'Dashboard';
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
                                            SUM(asset_value - deposit_value) AS gain,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
                                        FROM
                                            asset_log
                                        WHERE
                                            asset_id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth
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
                $this->pageTitle = "Asset View - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
                $vars['asset_class'] = $entity->getClass();
                $vars['asset_class_id'] = $entity->getClassID();
                $vars['closed'] = $entity->isClosed();
            }
        } elseif ($vars['type'] == 'class') {
            $dataQuery = $this->db->query("SELECT
                                            SUM(deposit_value) AS deposit_total,
                                            SUM(asset_value) AS asset_total,
                                            SUM(asset_value - deposit_value) AS gain,
                                            DATE_FORMAT(epoch, '%b %Y') AS period,
                                            EXTRACT(YEAR_MONTH
                                        FROM
                                            epoch) AS yearMonth
                                        FROM
                                            asset_log
                                        LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
                                        LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
                                        WHERE
                                            asset_classes.id ".$vars['modifier']." :item_id
                                        GROUP BY
                                            period,
                                            yearMonth
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
                $this->pageTitle = "Asset View - ".$entity->getDescription();
                $vars['page_title'] = $entity->getDescription();
            }
        }

        $vars['hide_total_pct'] = true;

        $vars['payments'] = [];
        while ($payment = $this->db->fetch($payQuery)) {
            if (array_key_exists($payment['yearMonth'],$vars['payments'])) {
                $vars['payments'][$payment['yearMonth']] += $payment['amount'];
            } else {
                $vars['payments'][$payment['yearMonth']] = $payment['amount'];
            }
        }

        $payTally = 0;
        while ($period = $this->db->fetch($dataQuery)) {
            $period['gain_delta'] = 0;
            $period['value_delta'] = 0;
            $period['growth_annualized'] = 0;
            $period['growth_factor'] = 0;
            $period['twr'] = 0;
            $period['twr_str'] = 0;
            $period['growth_factor_adj'] = 0;
            $period['twr_adj'] = 0;
            $period['twr_str_adj'] = 0;

            if (array_key_exists($period['yearMonth'],$vars['payments'])) {
                $payTally += $vars['payments'][$period['yearMonth']];
            }
            $period['payments'] = $payTally;
            $period['asset_total_adj'] = $period['asset_total'] + $payTally;

            if (isset($last)) {
                $period['gain_delta'] = number_format($period['gain'] - $last['gain'], 2, '.', '');
                $period['value_delta'] = number_format($period['deposit_total'] - $last['deposit_total'], 2, '.', '');

                if (($period['asset_total'] != 0) && ($last['asset_total'] != 0)) {
                    $period['growth_factor'] = $period['asset_total'] / ($last['asset_total'] + $period['value_delta']);

                    $period['growth_factor_adj'] = $period['asset_total_adj'] / ($last['asset_total_adj'] + $period['value_delta']);

                    if ($last['twr'] == 0) {
                        $period['twr'] = $period['growth_factor'];
                    } else {
                        $period['twr'] = $last['twr'] * $period['growth_factor'];
                    }
                    if ($period['twr'] != 0) {
                        $period['twr_str'] = ($period['twr'] - 1) * 100;
                        $period['twr_str'] = number_format($period['twr_str'], 2, '.', '');
                    }

                    if ($last['twr_adj'] == 0) {
                        $period['twr_adj'] = $period['growth_factor_adj'];
                    } else {
                        $period['twr_adj'] = $last['twr_adj'] * $period['growth_factor_adj'];
                    }
                    if ($period['twr_adj'] != 0) {
                        $period['twr_str_adj'] = ($period['twr_adj'] - 1) * 100;
                        $period['twr_str_adj'] = number_format($period['twr_str_adj'], 2, '.', '');
                    }

                }

                if ($last['asset_total'] != 0) {
                    $period['growth_annualized'] = pow($period['growth_factor_adj'], 12) - 1;
                    $period['growth_annualized'] = number_format($period['growth_annualized'] * 100, 2, '.', '');
                }
            }

            if ($period['deposit_total'] != 0) {
                $period['growth'] = number_format(($period['gain'] / $period['deposit_total']) * 100, 2, '.', '');
                //$vars['hide_total_pct'] = false;
            } else {
                if ($period['gain'] > 0) {
                    $period['growth'] = 100;
                } else {
                    $period['growth'] = 0;
                }
            }
            $period['growth_str'] = number_format($period['growth'],2);
            $vars['periodData'][] = $period;

            $last = $period;
        }

        if (isset($last)) {  // Probably no entries if this doesnt pass
            $last['deposit_str'] = $this->prettify($last['deposit_total']);
            $last['asset_str'] = $this->prettify($last['asset_total']);
            $last['gain_str'] = $this->prettify($last['gain']);
            $last['twr_str'] = number_format($last['twr_str'],2);
            $last['twr_str_adj'] = number_format($last['twr_str_adj'],2);
            $vars['mostRecent'] = $last;

            /////////// STANDARD DEVIATION ///////////
            $values = count($vars['periodData']);
            $x = 0;
            foreach ($vars['periodData'] as $data){
                // TODO: Move this to main loop ^^^
                $x = $x + $data['growth_annualized'];
            }
            $mean = $x / $values;
            $x = 0;
            foreach ($vars['periodData'] as $data){
                $deviation = pow($data['growth_annualized'] - $mean, 2);
                $x = $x + $deviation;
            }
            $variance = $x / $values;
            $vars['std_dev'] = sqrt($variance);
            /////////// STANDARD DEVIATION ///////////

        } else {
            $last['deposit_str'] = $this->prettify('0');
            $last['asset_str'] = $this->prettify('0');
            $last['gain_str'] = $this->prettify('0');
            $last['growth_str'] = '0.00';
            $last['twr_str'] = '0.00';
            $last['twr_str_adj'] = '0.00';
            $vars['mostRecent'] = $last;
        }

        //$this->setRegister('script', "https://cdnjs.cloudflare.com/ajax/libs/Chart.js/3.4.0/chart.min.js");
        $this->vars = $vars;
        $this->document = $twig->load('item_view.html');
    }

    private function prettify($value) {
        return ($value < 0 ? '-£'.number_format($value * -1, 2) : '£'.number_format($value, 2));
        //TODO some kind of configurable currency
    }

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
