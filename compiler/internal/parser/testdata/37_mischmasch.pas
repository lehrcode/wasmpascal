PROGRAM GrossUndKlein;
{ Groß-/Kleinschreibung ist egal, (* Kommentare *) duerfen sich mischen }
(* auch { so } herum *)
CONST
  MaxWert = $7FFF;
  Minimum = -MaxWert - 1;
  Mitte = (MaxWert + Minimum) DIV 2;
TYPE
  TBereich = Minimum..MaxWert;
  TKlein = 'a'..'z';
  TIndex = 1..10;
  TDatei = FILE OF TBereich;
  TFeld = ARRAY[TIndex, Boolean, TKlein] OF Byte;
  TSatz = PACKED RECORD a: Byte; b: Char END;
  TMenge = PACKED SET OF TKlein;
VAR
  Wert: TBereich;
  Zeichen: TKlein;
BEGIN
  Wert := Mitte;
  Zeichen := 'q';
  IF Wert >= MaxWert THEN WriteLn('max') ELSE IF Wert <= Minimum THEN WriteLn('min');
  WriteLn(Wert, Zeichen)
END.
