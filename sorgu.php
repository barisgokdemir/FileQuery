<?php

$gelen = $_GET["sorgu"];

$parca = explode(" ", $gelen);

echo $parca[0] . '<br>';
include 'search.php';

if ($parca[0] == 'select') { // select * from c: where name= deger 
    // parça[0] [1] [2] [3] [4] [5] [6]
        if ($parca[2] == 'from') {
            $dizinsorgu = $parca[3];

            echo $parca[4];
            if ($parca[4] == 'where') {
                // Where varsa bunlar var mı?
                    if ($parca[1] == '*' || $parca[1] == 'name' || $parca[1] == 'size' || $parca[1] == 'name,size' || $parca[1] == 'size,name') 
                    {
                        switch ($parca[5]) {

                            case 'name=':
                                echo 'select  from where name '.$dizinsorgu ;// $parça[6] 
                                    $search_key = $parca[6];
                                    ara_dizini($dizinsorgu);
                                break;
                            case 'type':
                                echo 'select * from where type geldim';
                                $search_type=$parca[6];
                                ara_dizini_type($dizinsorgu);
                                break;
                            case 'date':
                                echo 'select name date';
                                $search_type=$parca[6];
                                ara_dizini_date($dizinsorgu);
                                break;
                            case 'stat':
                                echo 'select stat';
                                $search_size=$parca[6];
                                ara_dizini_stat($dizinsorgu);
                                break;                               

                            case 'size':
                                echo 'select size';
                                $size_equals=$parca[6];
                                $search_size=$parca[7];
                                $size_type=strtoupper($parca[8]);
                                ara_dizini_boyut($dizinsorgu);
                                break;                               
                            case 'size<=':
                                echo 'select name size';
                                break;
                            case 'size<';
                                echo 'select size name';
                                break;
                            case 'size>';
                                echo 'select size name';
                                break;
                            case 'size>=';
                                echo 'select size name';
                                break;

                            default:
                            echo'yazım kurallarina dikat ediniz';
                                
                        }
                    }
                    else{
                        echo'yazım kurallarina dikat ediniz';
                    }   

            } else {
                // where yoksa burası çalışır
                    switch ($parca[4]) {

                        case 'name=':
                            echo 'select  from  name '.$dizinsorgu ;// $parça[6] 
                                $search_key = $parca[5];
                                ara_dizini($dizinsorgu);
                            break;
                        case 'type':
                            echo 'select * from where type geldim';
                            $search_type=$parca[5];
                            ara_dizini_type($dizinsorgu);
                            break;
                        case 'date':
                            echo 'select name date';
                            $search_type=$parca[5];
                            ara_dizini_date($dizinsorgu);
                            break;
                        case 'stat':
                            echo 'select stat';
                            $search_size=$parca[5];
                            ara_dizini_stat($dizinsorgu);
                            break;          
                        default:
                        echo'yazım kurallarina dikat ediniz';

                            
                    }
                }
        }
        else{    
            echo'yazım kurallarina dikat ediniz';


        }
}
else if ($parca[0] == 'insert') {
    
   

    if ($parca[2] == 'values') {
        $konum=$parca[1].$parca[3];
        fopen($konum, "w");
       echo 'dosya başarıyla oluşturuldu'.$parca[3];
    }
    else{
        echo'yazım kurallarina dikat ediniz';

    }
}
else if($parca[0] == 'delete') {

    if ($parca[2] == 'where') {
        echo 'silinen konum' . $parca[1];
        unlink($parca[1]);
    }
    else{
        echo'yazım kurallarina dikat ediniz';
    }
}
else{
$dizin = "C:/Users/Aygun/Desktop";
$search_key = $gelen;
ara_dizini($dizin);
}